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
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/config"
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

// BuildSpawnArgs assembles CLI arguments for spawning claude, respecting config.
// Returns the args slice and an optional cleanup function (for temp MCP config file).
func BuildSpawnArgs(cfg config.Config, prompt string, addDirs []string) ([]string, func()) {
	maxTurns := cfg.Spawn.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 15
	}
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--max-turns", strconv.Itoa(maxTurns),
		"--no-session-persistence",
	}
	if os.Getuid() != 0 {
		args = append(args, "--dangerously-skip-permissions")
	} else {
		// Root cannot use --dangerously-skip-permissions; pre-allow tools instead
		allowed := []string{
			"Bash", "Read", "Write", "Edit", "Glob", "Grep",
			"WebSearch", "WebFetch", "TodoWrite", "Task",
			"NotebookEdit", "NotebookRead",
		}
		// Include MCP tools from config
		if cfg.MCPServers != nil {
			for name, s := range cfg.MCPServers {
				if s.Enabled {
					allowed = append(allowed, "mcp__"+name)
				}
			}
		}
		args = append(args, "--allowedTools")
		args = append(args, allowed...)
	}
	if cfg.Spawn.Model != "" {
		args = append(args, "--model", cfg.Spawn.Model)
	}
	if cfg.Spawn.Effort != "" {
		args = append(args, "--effort", cfg.Spawn.Effort)
	}
	for _, dir := range addDirs {
		args = append(args, "--add-dir", dir)
	}

	var cleanup func()
	if cfg.MCPServers != nil {
		enabled := config.FilterEnabledMCP(cfg.MCPServers)
		if len(enabled) > 0 {
			tmpPath, err := config.BuildMCPConfigJSON(enabled)
			if err == nil {
				args = append(args, "--mcp-config", tmpPath)
				cleanup = func() { os.Remove(tmpPath) }
			}
		}
	}
	return args, cleanup
}

func runTask(ctx context.Context, task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg)) {
	emit := func(level logs.LogLevel, msg string, agent string) {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  task.ID,
			Project: task.Project,
			Message: msg,
			Level:   level,
			Agent:   agent,
		}})
	}

	emit(logs.LevelInfo, fmt.Sprintf("Autopilot started: %s", task.Description), "")
	if err := queue.UpdateState(task.ID, queue.StateRunning); err != nil {
		emit(logs.LevelError, fmt.Sprintf("State update failed: %v", err), "")
	}
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateRunning})

	addDirs := p.Dependencies
	var stepSummaries []string

	total := len(p.Steps)
	for _, step := range p.Steps {
		agent := step.Agent
		if agent == "" {
			agent = "dev"
		}

		if ctx.Err() != nil {
			emit(logs.LevelWarn, "Autopilot stopped by user", agent)
			queue.UpdateState(task.ID, queue.StatePlanned)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
			return
		}

		success := false
		var recoveryCtx string
		var lastRes spawnResult
		for retry := 0; retry < maxRetries; retry++ {
			if ctx.Err() != nil {
				emit(logs.LevelWarn, "Autopilot stopped by user", agent)
				queue.UpdateState(task.ID, queue.StatePlanned)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
				return
			}

			if retry == 0 {
				emit(logs.LevelInfo, fmt.Sprintf("Step %d/%d: %s", step.Number, total, step.Title), agent)
			} else {
				emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d: retry %d/%d", step.Number, total, retry, maxRetries-1), agent)
			}

			prompt := buildStepPrompt(task, p, step, retry, recoveryCtx, strings.Join(stepSummaries, "\n"))
			res, err := spawnClaude(ctx, task.Project, prompt, send, task.ID, addDirs, agent, cfg)
			lastRes = res

			if ctx.Err() != nil {
				emit(logs.LevelWarn, "Autopilot stopped by user", agent)
				queue.UpdateState(task.ID, queue.StatePlanned)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
				return
			}

			if err != nil {
				emit(logs.LevelError, fmt.Sprintf("Step %d/%d: spawn error: %v", step.Number, total, err), agent)
			}

			// Log output tail on failure for diagnostics
			if res.ExitCode != 0 {
				tail := res.Output
				if len(tail) > 200 {
					tail = tail[len(tail)-200:]
				}
				tail = strings.TrimSpace(tail)
				if tail != "" {
					emit(logs.LevelError, fmt.Sprintf("Step %d/%d output: %s", step.Number, total, tail), agent)
				}
			}

			// Check real success: exit 0 AND no permission denials
			stepOK := res.ExitCode == 0 && len(res.Denials) == 0
			if stepOK {
				// ReadOnly steps don't require write tools
				if !step.ReadOnly && !hasWriteTools(res.ToolsUsed) && retry < maxRetries-1 {
					emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d: no changes produced (tools: %v), retrying", step.Number, total, res.ToolsUsed), agent)
					recoveryCtx = "Previous attempt exited successfully but made NO file changes. You MUST create or edit files this time."
					continue
				}
				emit(logs.LevelSuccess, fmt.Sprintf("Step %d/%d complete (tools: %v)", step.Number, total, res.ToolsUsed), agent)
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
					step.Number, total, res.ExitCode, len(res.Denials)), agent)
				recoveryPrompt := buildRecoveryPrompt(task, step, res.Output, res.ExitCode)
				recRes, _ := spawnClaude(ctx, task.Project, recoveryPrompt, send, task.ID, addDirs, agent, cfg)
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
			emit(logs.LevelError, "BLOCKED: "+reason, agent)
			queue.SetBlockReason(task.ID, reason)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StateBlocked, Message: reason})
			return
		}
	}

	emit(logs.LevelSuccess, "All steps complete", "")
	if err := queue.UpdateState(task.ID, queue.StateDone); err != nil {
		emit(logs.LevelError, fmt.Sprintf("State update failed: %v", err), "")
	}
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateDone})
}

func buildStepPrompt(task queue.Task, p plan.Plan, step plan.Step, retry int, recoveryCtx, prevSteps string) string {
	home, _ := os.UserHomeDir()
	projectPath := filepath.Join(home, "Projects", task.Project)

	var sb strings.Builder
	if step.Agent != "" {
		sb.WriteString(fmt.Sprintf("You are the %s agent executing step %d of %d in an autopilot task.\n\n", step.Agent, step.Number, len(p.Steps)))
	} else {
		sb.WriteString(fmt.Sprintf("You are executing step %d of %d in an autopilot task.\n\n", step.Number, len(p.Steps)))
	}
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Description))
	sb.WriteString(fmt.Sprintf("Project: %s\n", task.Project))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", projectPath))
	sb.WriteString(fmt.Sprintf("Projects root: %s\n\n", filepath.Join(home, "Projects")))

	// Inject project context files if present
	for _, cf := range []struct {
		file  string
		title string
		limit int
	}{
		{"CLAUDE.md", "Project Guidelines (from CLAUDE.md)", 2000},
		{"README.md", "Project README (from README.md)", 1000},
		{"INSTALL.md", "Project Install Guide (from INSTALL.md)", 800},
		{"MEMORY.md", "Project Memory (from MEMORY.md)", 1500},
		{"CONTEXT.md", "Project Context (from CONTEXT.md)", 1500},
		{"AGENTS.md", "Project Agents (from AGENTS.md)", 1000},
		{"CHANGELOG.md", "Recent Changes (from CHANGELOG.md)", 800},
		{"VERSIONING.md", "Versioning Strategy (from VERSIONING.md)", 800},
		{"CONTRIBUTING.md", "Contributing Guidelines (from CONTRIBUTING.md)", 800},
	} {
		cfPath := filepath.Join(projectPath, cf.file)
		if data, err := os.ReadFile(cfPath); err == nil {
			content := string(data)
			if len(content) > cf.limit {
				content = content[:cf.limit] + "\n[truncated]"
			}
			sb.WriteString(fmt.Sprintf("## %s:\n", cf.title))
			sb.WriteString(content + "\n\n")
		}
	}

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
	if step.ReadOnly {
		sb.WriteString("\n1. This is a READ-ONLY step. You may ONLY read files, search code, and gather information. Do NOT create, edit, or modify any files.")
		sb.WriteString("\n2. Summarize your findings clearly so subsequent steps can use them.")
	} else {
		sb.WriteString("\n1. You MUST create, edit or modify source code files. Reading alone is FAILURE.")
		sb.WriteString("\n2. You have FULL permissions on all paths. For files outside your working directory use Bash (cp, tee, sed).")
		sb.WriteString("\n3. When done, list every file you created or modified.")
		sb.WriteString("\n4. If a previous step should have created something and didn't, do it yourself.")
	}
	sb.WriteString("\n5. NEVER create documentation files (.md) unless the task explicitly requests documentation. No SUMMARY.md, RESULTS.md, ANALYSIS.md, REPORT.md, etc.")
	sb.WriteString("\n6. NEVER invoke /bmad:core:workflows:party-mode or any /bmad slash command. You are already running inside the autopilot system. Invoking skills or workflows will break execution.")
	sb.WriteString("\n7. NEVER use EnterPlanMode or create plan files. You ARE the plan execution. Just do the work.")
	sb.WriteString("\n8. Be concise. Do not narrate what you are about to do. Just do it.")
	sb.WriteString("\n9. ALWAYS work on the dev branch. If not on dev, run: git checkout dev")
	return sb.String()
}

func buildRecoveryPrompt(task queue.Task, step plan.Step, output string, exitCode int) string {
	home, _ := os.UserHomeDir()
	projectPath := filepath.Join(home, "Projects", task.Project)
	truncated := output
	if len(truncated) > 1000 {
		truncated = truncated[len(truncated)-1000:]
	}
	return fmt.Sprintf(
		"A step execution failed. Analyze the failure and apply a targeted fix.\n\n"+
			"Project: %s\nWorking directory: %s\n"+
			"Task: %s\nStep %d: %s\nExit code: %d\n\n"+
			"Recent output (last 1000 chars):\n%s\n\n"+
			"INSTRUCTIONS:\n"+
			"1. Identify the root cause from the output above\n"+
			"2. Apply the minimal fix needed (edit files, run commands)\n"+
			"3. Do NOT repeat the entire step — only fix what failed\n"+
			"4. List every file you modified\n",
		task.Project, projectPath,
		task.Description, step.Number, step.Title, exitCode, truncated,
	)
}

func spawnClaude(ctx context.Context, project, prompt string, send func(tea.Msg), taskID int, addDirs []string, agent string, cfg config.Config) (spawnResult, error) {
	home, _ := os.UserHomeDir()
	projectPath := filepath.Join(home, "Projects", project)

	if _, err := os.Stat(projectPath); err != nil {
		projectPath = home
	}

	// Apply step timeout if configured
	spawnCtx := ctx
	var cancelTimeout context.CancelFunc
	if cfg.Spawn.StepTimeoutMin > 0 {
		spawnCtx, cancelTimeout = context.WithTimeout(ctx, time.Duration(cfg.Spawn.StepTimeoutMin)*time.Minute)
	}
	if cancelTimeout != nil {
		defer cancelTimeout()
	}

	env := filterEnv(os.Environ(), "CLAUDECODE")

	// Satirical git identity for autopilot commits
	gitName := GenerateName()
	gitEmail := GenerateEmail(gitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+gitName,
		"GIT_AUTHOR_EMAIL="+gitEmail,
		"GIT_COMMITTER_NAME="+gitName,
		"GIT_COMMITTER_EMAIL="+gitEmail,
	)

	args, cleanup := BuildSpawnArgs(cfg, prompt, addDirs)
	if cleanup != nil {
		defer cleanup()
	}

	cmd := exec.CommandContext(spawnCtx, "claude", args...)
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
							Agent:   agent,
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
					Agent:   agent,
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
					Agent:   agent,
				}})
			}
		}
	}

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		// Detect step timeout (spawnCtx expired but parent ctx still alive)
		if spawnCtx.Err() != nil && ctx.Err() == nil {
			send(LogMsg{Entry: logs.LogEntry{
				Time:    time.Now(),
				TaskID:  taskID,
				Project: project,
				Message: fmt.Sprintf("Step timed out after %d min", cfg.Spawn.StepTimeoutMin),
				Level:   logs.LevelError,
				Agent:   agent,
			}})
			return spawnResult{ExitCode: 124, Output: fullOutput.String()}, fmt.Errorf("step timeout after %d min", cfg.Spawn.StepTimeoutMin)
		}
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
