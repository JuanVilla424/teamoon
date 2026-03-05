package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/backend"
	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/projectinit"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

const maxRetries = 3

func runTask(ctx context.Context, task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg), b backend.Backend) {
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
		var lastRes backend.SpawnResult
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
			res, err := spawnBackend(ctx, b, task.Project, prompt, send, task.ID, addDirs, agent, cfg, sessionID)
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
				recRes, _ := spawnBackend(ctx, b, task.Project, recoveryPrompt, send, task.ID, addDirs, agent, cfg, sessionID)
				// Feed recovery analysis as context to next retry
				recoveryCtx = failInfo.String()
				if recRes.Output != "" {
					recoveryCtx += "\nRecovery analysis:\n" + backend.ExtractResult(recRes.Output)
				}
			}
		}

		if success {
			queue.SetCurrentStep(task.ID, step.Number)
		}

		// Accumulate step context for subsequent steps
		if success {
			summary := backend.ExtractResult(lastRes.Output)
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

// spawnBackend executes a single step via the Backend interface.
func spawnBackend(ctx context.Context, b backend.Backend, project, prompt string, send func(tea.Msg), taskID int, addDirs []string, agent string, cfg config.Config, sessionID string) (backend.SpawnResult, error) {
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

	// Build environment
	env := backend.FilterEnv(os.Environ(), "CLAUDECODE")
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

	// Build MCP config
	mcpConfig, mcpTools, mcpCleanup := backend.BuildMCPArgs(cfg)
	if mcpCleanup != nil {
		defer mcpCleanup()
	}

	// Build allowed tools for root mode
	var allowed []string
	if os.Getuid() == 0 && b.Caps().ToolFiltering {
		allowed = []string{
			"Bash", "Read", "Write", "Edit", "Glob", "Grep",
			"WebSearch", "WebFetch", "TodoWrite", "Task",
			"NotebookEdit", "NotebookRead",
		}
		allowed = append(allowed, mcpTools...)
	}

	req := backend.SpawnRequest{
		Prompt:     prompt,
		ProjectDir: projectPath,
		WorkDir:    projectPath,
		AddDirs:    addDirs,
		SessionID:  sessionID,
		Model:      b.ResolveModel(cfg.Spawn.Model, "exec"),
		Effort:     cfg.Spawn.Effort,
		MaxTurns:   cfg.Spawn.MaxTurns,
		Env:        env,
		Phase:      "exec",
		DisallowedTools: []string{
			"AskUserQuestion", "EnterPlanMode", "ExitPlanMode", "TodoWrite",
		},
	}

	if b.Caps().MCPConfig {
		req.MCPConfig = mcpConfig
	}
	if b.Caps().ToolFiltering && len(allowed) > 0 {
		req.AllowedTools = allowed
	}

	// Execute via backend
	events := make(chan backend.Event, 64)
	var result backend.SpawnResult
	var execErr error

	go func() {
		result, execErr = b.Execute(spawnCtx, req, events)
	}()

	for ev := range events {
		if formatted := backend.FormatEvent(ev); formatted != "" {
			level := logs.LevelInfo
			if ev.Type == "error" || ev.IsError {
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
	}

	// Detect step timeout
	if spawnCtx.Err() != nil && ctx.Err() == nil {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  taskID,
			Project: project,
			Message: fmt.Sprintf("Step timed out after %d min", cfg.Spawn.StepTimeoutMin),
			Level:   logs.LevelError,
			Agent:   agent,
		}})
		return backend.SpawnResult{ExitCode: 124, Output: result.Output}, fmt.Errorf("step timeout after %d min", cfg.Spawn.StepTimeoutMin)
	}

	return result, execErr
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
	sb.WriteString("\n12. Commits: single line, NO Co-Authored-By, NO 'Made by Claude', NO 'Generated with Claude'. Format: type(core): description [versioning keyword]. Versioning keyword is MANDATORY: feat→[minor candidate], fix→[patch candidate], refactor/docs/style/test/chore→[patch candidate], breaking changes→[major candidate].")
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
