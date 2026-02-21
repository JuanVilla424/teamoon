package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

const maxRetries = 3

type streamEvent struct {
	Type              string              `json:"type"`
	Subtype           string              `json:"subtype,omitempty"`
	Message           *streamMessage      `json:"message,omitempty"`
	Result            string              `json:"result,omitempty"`
	Error             *streamError        `json:"error,omitempty"`
	IsError           bool                `json:"is_error,omitempty"`
	PermissionDenials []permissionDenial  `json:"permission_denials,omitempty"`
}

type streamMessage struct {
	Content []streamContent `json:"content"`
}

type streamContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
}

type streamError struct {
	Message string `json:"message"`
}

type permissionDenial struct {
	ToolName string `json:"tool_name"`
}

type spawnResult struct {
	ExitCode  int
	Output    string
	Denials   []string
	ToolsUsed []string
}

func runTask(ctx context.Context, task queue.Task, p plan.Plan, send func(tea.Msg)) {
	emit := func(level logs.LogLevel, msg string) {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  task.ID,
			Project: task.Project,
			Message: msg,
			Level:   level,
		}})
	}

	emit(logs.LevelInfo, fmt.Sprintf("Autopilot started: %s", task.Description))
	queue.UpdateState(task.ID, queue.StateRunning)
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateRunning})

	addDirs := p.Dependencies
	var stepSummaries []string

	total := len(p.Steps)
	for _, step := range p.Steps {
		if ctx.Err() != nil {
			emit(logs.LevelWarn, "Autopilot stopped by user")
			queue.UpdateState(task.ID, queue.StatePlanned)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
			return
		}

		success := false
		var recoveryCtx string
		var lastRes spawnResult
		for retry := 0; retry < maxRetries; retry++ {
			if ctx.Err() != nil {
				emit(logs.LevelWarn, "Autopilot stopped by user")
				queue.UpdateState(task.ID, queue.StatePlanned)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
				return
			}

			if retry == 0 {
				emit(logs.LevelInfo, fmt.Sprintf("Step %d/%d: %s", step.Number, total, step.Title))
			} else {
				emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d: retry %d/%d", step.Number, total, retry, maxRetries-1))
			}

			prompt := buildStepPrompt(task, p, step, retry, recoveryCtx, strings.Join(stepSummaries, "\n"))
			res, err := spawnClaude(ctx, task.Project, prompt, send, task.ID, addDirs)
			lastRes = res

			if ctx.Err() != nil {
				emit(logs.LevelWarn, "Autopilot stopped by user")
				queue.UpdateState(task.ID, queue.StatePlanned)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
				return
			}

			if err != nil {
				emit(logs.LevelError, fmt.Sprintf("Step %d/%d: spawn error: %v", step.Number, total, err))
			}

			// Log output tail on failure for diagnostics
			if res.ExitCode != 0 {
				tail := res.Output
				if len(tail) > 200 {
					tail = tail[len(tail)-200:]
				}
				tail = strings.TrimSpace(tail)
				if tail != "" {
					emit(logs.LevelError, fmt.Sprintf("Step %d/%d output: %s", step.Number, total, tail))
				}
			}

			// Check real success: exit 0 AND no permission denials
			stepOK := res.ExitCode == 0 && len(res.Denials) == 0
			if stepOK {
				// Verify actual changes were produced (not just reading)
				if !hasWriteTools(res.ToolsUsed) && retry < maxRetries-1 {
					emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d: no changes produced (tools: %v), retrying", step.Number, total, res.ToolsUsed))
					recoveryCtx = "Previous attempt exited successfully but made NO file changes. You MUST create or edit files this time."
					continue
				}
				emit(logs.LevelSuccess, fmt.Sprintf("Step %d/%d complete (tools: %v)", step.Number, total, res.ToolsUsed))
				success = true
				break
			}

			// Build failure context for Layer 2
			var failInfo strings.Builder
			if res.ExitCode != 0 {
				failInfo.WriteString(fmt.Sprintf("Exit code: %d\n", res.ExitCode))
			}
			if len(res.Denials) > 0 {
				failInfo.WriteString("Permission denials: " + strings.Join(res.Denials, ", ") + "\n")
				failInfo.WriteString("Use Glob/Read tools instead of denied tools for file access.\n")
			}

			// Layer 2: Deliberative — analyze failure and feed context to next retry
			if retry < maxRetries-1 {
				emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d failed (exit %d, %d denials), analyzing...",
					step.Number, total, res.ExitCode, len(res.Denials)))
				recoveryPrompt := buildRecoveryPrompt(task, step, res.Output, res.ExitCode)
				recRes, _ := spawnClaude(ctx, task.Project, recoveryPrompt, send, task.ID, addDirs)
				// Feed recovery analysis as context to next retry
				recoveryCtx = failInfo.String()
				if recRes.Output != "" {
					// Extract the result text from recovery for context
					recoveryCtx += "\nRecovery analysis:\n" + extractResult(recRes.Output)
				}
			}
		}

		// Accumulate step context for subsequent steps
		if success {
			summary := extractResult(lastRes.Output)
			if summary != "" {
				if len(summary) > 300 {
					summary = summary[:300]
				}
				stepSummaries = append(stepSummaries, fmt.Sprintf("Step %d: %s", step.Number, summary))
			}
		}

		if !success {
			// Layer 3: Meta-cognitive — block task
			reason := fmt.Sprintf("Step %d '%s' failed after %d attempts", step.Number, step.Title, maxRetries)
			emit(logs.LevelError, "BLOCKED: "+reason)
			queue.SetBlockReason(task.ID, reason)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StateBlocked, Message: reason})
			return
		}
	}

	emit(logs.LevelSuccess, "All steps complete")
	queue.UpdateState(task.ID, queue.StateDone)
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateDone})
}

func buildStepPrompt(task queue.Task, p plan.Plan, step plan.Step, retry int, recoveryCtx, prevSteps string) string {
	home, _ := os.UserHomeDir()
	projectPath := filepath.Join(home, "Projects", task.Project)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are executing step %d of %d in an autopilot task.\n\n", step.Number, len(p.Steps)))
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Description))
	sb.WriteString(fmt.Sprintf("Project: %s\n", task.Project))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", projectPath))
	sb.WriteString(fmt.Sprintf("Projects root: %s\n\n", filepath.Join(home, "Projects")))

	if prevSteps != "" {
		sb.WriteString("Previous steps completed:\n" + prevSteps + "\n\n")
	}

	sb.WriteString(fmt.Sprintf("Step %d: %s\n", step.Number, step.Title))
	sb.WriteString(step.Body + "\n")
	if step.Verify != "" {
		sb.WriteString(fmt.Sprintf("\nVerify when done: %s\n", step.Verify))
	}
	if retry > 0 && recoveryCtx != "" {
		sb.WriteString(fmt.Sprintf("\nPrevious attempt context:\n%s\n", recoveryCtx))
	}

	sb.WriteString("\nRULES:")
	sb.WriteString("\n1. You MUST create, edit or modify files. Reading alone is FAILURE.")
	sb.WriteString("\n2. You have FULL permissions on all paths. For files outside your working directory use Bash (cp, tee, sed).")
	sb.WriteString("\n3. When done, list every file you created or modified.")
	sb.WriteString("\n4. If a previous step should have created something and didn't, do it yourself.")
	return sb.String()
}

func buildRecoveryPrompt(task queue.Task, step plan.Step, output string, exitCode int) string {
	truncated := output
	if len(truncated) > 500 {
		truncated = truncated[len(truncated)-500:]
	}
	return fmt.Sprintf(
		"A step execution failed. Analyze and fix the issue.\n\n"+
			"Task: %s\nStep: %s\nExit code: %d\nRecent output:\n%s\n\n"+
			"Diagnose the root cause and apply a fix.",
		task.Description, step.Title, exitCode, truncated,
	)
}

func spawnClaude(ctx context.Context, project, prompt string, send func(tea.Msg), taskID int, addDirs []string) (spawnResult, error) {
	home, _ := os.UserHomeDir()
	projectPath := filepath.Join(home, "Projects", project)

	if _, err := os.Stat(projectPath); err != nil {
		projectPath = home
	}

	env := filterEnv(os.Environ(), "CLAUDECODE")

	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--max-turns", "25",
		"--no-session-persistence",
		"--dangerously-skip-permissions",
	}
	for _, dir := range addDirs {
		args = append(args, "--add-dir", dir)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Env = env
	cmd.Dir = projectPath

	devNull, _ := os.Open(os.DevNull)
	if devNull != nil {
		cmd.Stdin = devNull
		defer devNull.Close()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return spawnResult{ExitCode: -1}, err
	}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return spawnResult{ExitCode: -1}, err
	}

	var fullOutput strings.Builder
	var denials []string
	var toolsUsed []string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fullOutput.WriteString(line + "\n")

		var event streamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, c := range event.Message.Content {
					if c.Type == "tool_use" && c.Name != "" {
						toolsUsed = append(toolsUsed, c.Name)
					}
					if c.Type == "text" && c.Text != "" {
						send(LogMsg{Entry: logs.LogEntry{
							Time:    time.Now(),
							TaskID:  taskID,
							Project: project,
							Message: c.Text,
							Level:   logs.LevelInfo,
						}})
					}
				}
			}
		case "result":
			// Capture permission denials from result event
			for _, d := range event.PermissionDenials {
				denials = append(denials, d.ToolName)
			}
			if event.Result != "" {
				send(LogMsg{Entry: logs.LogEntry{
					Time:    time.Now(),
					TaskID:  taskID,
					Project: project,
					Message: event.Result,
					Level:   logs.LevelInfo,
				}})
			}
		case "error":
			if event.Error != nil {
				send(LogMsg{Entry: logs.LogEntry{
					Time:    time.Now(),
					TaskID:  taskID,
					Project: project,
					Message: "Error: " + event.Error.Message,
					Level:   logs.LevelError,
				}})
			}
		}
	}

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return spawnResult{ExitCode: -1, Output: fullOutput.String()}, err
		}
	}

	if stderrBuf.Len() > 0 {
		fullOutput.WriteString("\n[stderr] " + stderrBuf.String())
	}

	return spawnResult{
		ExitCode:  exitCode,
		Output:    fullOutput.String(),
		Denials:   denials,
		ToolsUsed: toolsUsed,
	}, nil
}

func extractResult(rawOutput string) string {
	for _, line := range strings.Split(rawOutput, "\n") {
		var event streamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "result" && event.Result != "" {
			r := event.Result
			if len(r) > 300 {
				r = r[:300] + "..."
			}
			return r
		}
	}
	return ""
}

var writeToolSet = map[string]bool{
	"Write": true, "Edit": true, "Bash": true, "NotebookEdit": true,
}

func hasWriteTools(tools []string) bool {
	for _, t := range tools {
		if writeToolSet[t] {
			return true
		}
	}
	return false
}

func filterEnv(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}
