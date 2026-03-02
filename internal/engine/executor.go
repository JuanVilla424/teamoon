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
	"github.com/JuanVilla424/teamoon/internal/projectinit"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

const maxRetries = 3

// StreamEvent represents a single line from Claude CLI's stream-json output.
type StreamEvent struct {
	Type              string             `json:"type"`
	Subtype           string             `json:"subtype,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
	Message           *StreamMessage     `json:"message,omitempty"`
	Result            string             `json:"result,omitempty"`
	Error             *StreamError       `json:"error,omitempty"`
	IsError           bool               `json:"is_error,omitempty"`
	PermissionDenials []PermissionDenial `json:"permission_denials,omitempty"`
	ToolUseResult     *ToolUseResult     `json:"tool_use_result,omitempty"`
}

// StreamMessage is the message payload of an assistant or user event.
type StreamMessage struct {
	Content []StreamContent `json:"content"`
}

// StreamContent is a single content block within a stream message.
type StreamContent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
}

// StreamError is an error payload from the Claude CLI.
type StreamError struct {
	Message string `json:"message"`
}

// PermissionDenial records a tool that was denied by permission mode.
type PermissionDenial struct {
	ToolName string `json:"tool_name"`
}

// ToolUseResult contains stdout/stderr from a tool execution.
type ToolUseResult struct {
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

// FormatStreamEvent converts a raw stream-json event into human-readable console lines.
// Returns empty string if the event produces no visible output.
func FormatStreamEvent(event StreamEvent) string {
	var lines []string

	switch event.Type {
	case "assistant":
		if event.Message != nil {
			for _, c := range event.Message.Content {
				switch c.Type {
				case "tool_use":
					desc := FormatToolCall(c.Name, c.Input)
					lines = append(lines, "⏺ "+desc)
				case "text":
					if c.Text != "" {
						lines = append(lines, c.Text)
					}
				}
			}
		}
	case "user":
		if event.Message != nil {
			for _, c := range event.Message.Content {
				if c.Type == "tool_result" {
					content := c.Content
					if content == "" && event.ToolUseResult != nil {
						content = event.ToolUseResult.Stdout
					}
					if content != "" {
						if len(content) > 500 {
							content = content[:500] + "\n... (truncated)"
						}
						prefix := "  ↳ "
						if c.IsError {
							prefix = "  ✗ "
						}
						lines = append(lines, prefix+content)
					}
				}
			}
		}
	case "result":
		// result is captured separately for plan parsing; don't duplicate
	case "error":
		if event.Error != nil {
			lines = append(lines, "✗ Error: "+event.Error.Message)
		}
	case "system":
		if event.Subtype != "" {
			lines = append(lines, "⚙ "+event.Subtype)
		}
	}

	return strings.Join(lines, "\n")
}

func FormatToolCall(name string, input map[string]any) string {
	str := func(key string) string {
		if v, ok := input[key]; ok {
			if s, ok := v.(string); ok {
				if len(s) > 120 {
					return s[:120] + "..."
				}
				return s
			}
		}
		return ""
	}
	switch name {
	case "Read":
		if p := str("file_path"); p != "" {
			return "Read " + p
		}
	case "Write":
		if p := str("file_path"); p != "" {
			return "Write " + p
		}
	case "Edit":
		if p := str("file_path"); p != "" {
			return "Edit " + p
		}
	case "Glob":
		if p := str("pattern"); p != "" {
			return "Glob " + p
		}
	case "Grep":
		if p := str("pattern"); p != "" {
			path := str("path")
			if path != "" {
				return "Grep " + p + " in " + path
			}
			return "Grep " + p
		}
	case "Bash":
		if c := str("command"); c != "" {
			return "Bash: " + c
		}
	case "WebSearch":
		if q := str("query"); q != "" {
			return "WebSearch: " + q
		}
	case "WebFetch":
		if u := str("url"); u != "" {
			return "WebFetch: " + u
		}
	case "Task":
		if d := str("description"); d != "" {
			return "Task: " + d
		}
	case "TodoWrite":
		return "TodoWrite"
	case "Skill":
		if s := str("skill"); s != "" {
			return "Skill: " + s
		}
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.SplitN(name, "__", 3)
		if len(parts) >= 3 {
			return parts[1] + ": " + strings.ReplaceAll(parts[2], "-", " ")
		}
	}
	return name
}

type spawnResult struct {
	ExitCode  int
	Output    string
	Denials   []string
	ToolsUsed []string
	SessionID string
}

// BuildSpawnArgs assembles CLI arguments for spawning claude, respecting config.
// Returns the args slice and an optional cleanup function (for temp MCP config file).
// ResolveModel maps teamoon meta-models to full Claude CLI model IDs.
// "opusplan" is resolved based on the phase: "claude-opus-4-6" for planning, "claude-sonnet-4-6" for execution.
// Pass phase="plan" or phase="exec". Other model values are passed through unchanged.
func ResolveModel(model, phase string) string {
	if model == "opusplan" {
		if phase == "plan" || phase == "chat" {
			return "claude-opus-4-6"
		}
		return "claude-sonnet-4-6"
	}
	switch model {
	case "opus":
		return "claude-opus-4-6"
	case "sonnet":
		return "claude-sonnet-4-6"
	case "haiku":
		return "claude-haiku-4-5-20251001"
	}
	return model
}

func BuildSpawnArgs(cfg config.Config, prompt string, addDirs []string, sessionID string) ([]string, func()) {
	var args []string
	if sessionID != "" {
		args = []string{
			"--resume", sessionID,
			"-p", prompt,
			"--output-format", "stream-json",
			"--verbose",
		}
	} else {
		args = []string{
			"-p", prompt,
			"--output-format", "stream-json",
			"--verbose",
			"--no-session-persistence",
		}
	}
	// MaxTurns: >0 = explicit cap, 0 = unlimited (omit flag), <0 = safe default 15
	maxTurns := cfg.Spawn.MaxTurns
	if maxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(maxTurns))
	} else if maxTurns < 0 {
		args = append(args, "--max-turns", "15")
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
		// Include only essential MCP tools (context7, github)
		if cfg.MCPServers != nil {
			for name, s := range cfg.MCPServers {
				if s.Enabled && (name == "context7" || name == "github") {
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

	// Only pass essential MCP servers to spawned agents.
	// Plugins like claude-mem, memory, sequential-thinking generate events
	// that waste agent turns with "No action needed" responses.
	var cleanup func()
	if cfg.MCPServers != nil {
		essential := make(map[string]config.MCPServer)
		for name, s := range cfg.MCPServers {
			if s.Enabled && (name == "context7" || name == "github") {
				essential[name] = s
			}
		}
		if len(essential) > 0 {
			tmpPath, err := config.BuildMCPConfigJSON(essential)
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

	// Ensure .bmad symlink exists so BMAD workflows resolve @.bmad/ paths
	projectinit.EnsureBMADLink(filepath.Join(cfg.ProjectsDir, task.Project))

	emit(logs.LevelInfo, fmt.Sprintf("Autopilot started: %s", task.Description), "")
	if err := queue.UpdateState(task.ID, queue.StateRunning); err != nil {
		emit(logs.LevelError, fmt.Sprintf("State update failed: %v", err), "")
	}
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateRunning})

	// Clear any lingering plan-gen session IDs so restart recovery works correctly
	queue.SetSessionID(task.ID, "")

	addDirs := p.Dependencies
	var stepSummaries []string
	var sessionID string

	total := len(p.Steps)
	queue.SetTotalSteps(task.ID, total)
	for _, step := range p.Steps {
		// Skip steps already completed (resume after restart)
		if step.Number <= task.CurrentStep {
			emit(logs.LevelInfo, fmt.Sprintf("Step %d/%d: already completed, skipping", step.Number, total), step.Agent)
			continue
		}

		agent := step.Agent

		if ctx.Err() != nil {
			emit(logs.LevelWarn, "Autopilot stopped by user", agent)
			queue.UpdateState(task.ID, queue.StatePlanned)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
			return
		}

		for reason := CheckGuardrails(); reason != ""; reason = CheckGuardrails() {
			emit(logs.LevelWarn, "Guardrail: "+reason+", waiting 2m...", agent)
			select {
			case <-ctx.Done():
				queue.UpdateState(task.ID, queue.StatePlanned)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned, Message: "stopped"})
				return
			case <-time.After(2 * time.Minute):
			}
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

			queue.SetCurrentStep(task.ID, step.Number)
			if retry == 0 {
				emit(logs.LevelInfo, fmt.Sprintf("Step %d/%d: %s", step.Number, total, step.Title), agent)
			} else {
				emit(logs.LevelWarn, fmt.Sprintf("Step %d/%d: retry %d/%d", step.Number, total, retry, maxRetries-1), agent)
			}

			prompt := buildStepPrompt(task, p, step, retry, recoveryCtx, strings.Join(stepSummaries, "\n"), cfg)
			res, err := spawnClaude(ctx, task.Project, prompt, send, task.ID, addDirs, agent, cfg, sessionID)
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
				recoveryPrompt := buildRecoveryPrompt(task, step, res.Output, res.ExitCode, cfg)
				recRes, _ := spawnClaude(ctx, task.Project, recoveryPrompt, send, task.ID, addDirs, agent, cfg, sessionID)
				// Feed recovery analysis as context to next retry
				recoveryCtx = failInfo.String()
				if recRes.Output != "" {
					// Extract the result text from recovery for context
					recoveryCtx += "\nRecovery analysis:\n" + extractResult(recRes.Output)
				}
			}
		}

		if success {
			queue.SetCurrentStep(task.ID, step.Number)
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
			// Layer 3: Meta-cognitive — fail task
			reason := fmt.Sprintf("Step %d '%s' failed after %d attempts", step.Number, step.Title, maxRetries)
			emit(logs.LevelError, "FAILED: "+reason, agent)
			queue.SetFailReason(task.ID, reason)
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: reason})
			return
		}
	}

	emit(logs.LevelSuccess, "All steps complete", "")
	if err := queue.UpdateState(task.ID, queue.StateDone); err != nil {
		emit(logs.LevelError, fmt.Sprintf("State update failed: %v", err), "")
	}
	send(TaskStateMsg{TaskID: task.ID, State: queue.StateDone})
}

func buildStepPrompt(task queue.Task, p plan.Plan, step plan.Step, retry int, recoveryCtx, prevSteps string, cfg config.Config) string {
	if task.Assignee == "system" {
		return buildSystemStepPrompt(task, p, step, retry, recoveryCtx, prevSteps, cfg)
	}
	projectPath := filepath.Join(cfg.ProjectsDir, task.Project)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are the %s agent executing step %d of %d in an autopilot task.\n\n", step.Agent, step.Number, len(p.Steps)))
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Description))
	sb.WriteString(fmt.Sprintf("Project: %s\n", task.Project))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", projectPath))
	sb.WriteString(fmt.Sprintf("Projects root: %s\n\n", cfg.ProjectsDir))

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
		{"ARCHITECT.md", "Project Architecture (from ARCHITECT.md)", 1500},
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
	sb.WriteString("\n6. NEVER invoke /bmad slash commands (party-mode, brainstorming-session, or any /bmad:* workflow). Use skills like /using-superpowers, /frontend-design, /ui-ux-pro-max when they help the task.")
	sb.WriteString("\n7. NEVER use EnterPlanMode or create plan files. You ARE the plan execution. Just do the work.")
	sb.WriteString("\n8. Be concise. Do not narrate. Do not ask questions. Do not offer to do more. When done, STOP.")
	sb.WriteString("\n9. ALWAYS work on the dev branch. If not on dev, run: git checkout dev")
	sb.WriteString("\n10. NEVER say 'Is there anything else', 'Let me know', 'Ready when you are', or similar. Just finish and stop.")
	sb.WriteString("\n11. NEVER use heredoc (<<EOF, <<'EOF', cat <<) in any command. Use direct strings with quotes for git commit -m and similar.")
	sb.WriteString("\n12. Commits: single line, NO Co-Authored-By, NO 'Made by Claude', NO 'Generated with Claude'. Format: type(core): description")
	sb.WriteString("\n13. NEVER create pull requests with Claude/AI mentions in title or body. No 'Generated with Claude Code', no robot emojis, no AI attribution.")
	sb.WriteString("\n14. NEVER install packages directly (npm install pkg, pip install pkg). ALWAYS add to the manifest file first (package.json, requirements.txt, go.mod, etc.) then run the install command without package names.")
	sb.WriteString("\n15. When your work is DONE, output your final summary and STOP IMMEDIATELY. Do NOT respond to hook events, system messages, or session stop notifications. Do NOT say 'ok', 'acknowledged', 'nothing to do'. Just STOP.")
	return sb.String()
}

func buildRecoveryPrompt(task queue.Task, step plan.Step, output string, exitCode int, cfg config.Config) string {
	projectPath := filepath.Join(cfg.ProjectsDir, task.Project)
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

func buildSystemStepPrompt(task queue.Task, p plan.Plan, step plan.Step, retry int, recoveryCtx, prevSteps string, cfg config.Config) string {
	home, _ := os.UserHomeDir()
	var sb strings.Builder
	if step.Agent != "" {
		sb.WriteString(fmt.Sprintf("You are the %s agent executing system step %d of %d.\n\n", step.Agent, step.Number, len(p.Steps)))
	} else {
		sb.WriteString(fmt.Sprintf("You are executing system step %d of %d.\n\n", step.Number, len(p.Steps)))
	}
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Description))
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", home))
	sb.WriteString(fmt.Sprintf("Projects root: %s\n\n", cfg.ProjectsDir))
	if cfg.SudoEnabled {
		sb.WriteString("## Sudo\nSudo is ENABLED. You may use sudo for operations requiring elevated privileges.\n\n")
	} else {
		sb.WriteString("## Sudo\nSudo is DISABLED. Avoid commands requiring sudo.\n\n")
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
	sb.WriteString("\n1. You are performing SYSTEM ADMINISTRATION, not software development.")
	sb.WriteString("\n2. Be surgical — only change what the task requires.")
	sb.WriteString("\n3. Verify your changes after making them.")
	sb.WriteString("\n4. Security hooks remain active and block destructive operations.")
	sb.WriteString("\n5. Do NOT create documentation files, commit code, or run autopilot workflows.")
	sb.WriteString("\n6. NEVER invoke /bmad slash commands or EnterPlanMode.")
	sb.WriteString("\n7. Be concise. Do not narrate. Just do the work.")
	return sb.String()
}

func spawnClaude(ctx context.Context, project, prompt string, send func(tea.Msg), taskID int, addDirs []string, agent string, cfg config.Config, sessionID string) (spawnResult, error) {
	projectPath := filepath.Join(cfg.ProjectsDir, project)

	if _, err := os.Stat(projectPath); err != nil {
		home, _ := os.UserHomeDir()
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
	if cfg.SudoEnabled {
		env = append(env, "TEAMOON_SUDO_ENABLED=true")
	}

	// Satirical git identity for autopilot commits
	gitName := GenerateName()
	gitEmail := GenerateEmail(gitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+gitName,
		"GIT_AUTHOR_EMAIL="+gitEmail,
		"GIT_COMMITTER_NAME="+gitName,
		"GIT_COMMITTER_EMAIL="+gitEmail,
	)

	// Resolve meta-model for step execution phase
	execCfg := cfg
	execCfg.Spawn.Model = ResolveModel(cfg.Spawn.Model, "exec")
	args, cleanup := BuildSpawnArgs(execCfg, prompt, addDirs, sessionID)
	if cleanup != nil {
		defer cleanup()
	}
	// Block interactive tools — autopilot steps must be self-contained
	args = append(args, "--disallowedTools",
		"AskUserQuestion,EnterPlanMode,ExitPlanMode,TodoWrite")

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
	var capturedSessionID string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fullOutput.WriteString(line + "\n")

		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// Capture session_id from first event that has it
		if event.SessionID != "" && capturedSessionID == "" {
			capturedSessionID = event.SessionID
		}

		// Format and send to web UI as console output
		if formatted := FormatStreamEvent(event); formatted != "" {
			level := logs.LevelInfo
			if event.Type == "error" {
				level = logs.LevelError
			}
			send(LogMsg{Entry: logs.LogEntry{
				Time:    time.Now(),
				TaskID:  taskID,
				Project: project,
				Message: formatted,
				Level:   level,
				Agent:   agent,
			}})
		}

		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, c := range event.Message.Content {
					if c.Type == "tool_use" && c.Name != "" {
						toolsUsed = append(toolsUsed, c.Name)
					}
				}
			}
		case "result":
			for _, d := range event.PermissionDenials {
				denials = append(denials, d.ToolName)
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
		SessionID: capturedSessionID,
	}, nil
}

func extractResult(rawOutput string) string {
	for _, line := range strings.Split(rawOutput, "\n") {
		var event StreamEvent
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
