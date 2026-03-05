package plangen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JuanVilla424/teamoon/internal/backend"
	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/projectinit"
	"github.com/JuanVilla424/teamoon/internal/queue"
	"github.com/JuanVilla424/teamoon/internal/uploads"
)

const (
	maxAttachmentChars = 5000
	maxTotalChars      = 20000
)


// buildAttachmentBlock resolves task attachments and returns their text content as a prompt block.
func buildAttachmentBlock(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	atts := uploads.ResolveIDs(ids)
	if len(atts) == 0 {
		return ""
	}
	var sb strings.Builder
	totalChars := 0
	for _, a := range atts {
		if !uploads.IsTextMIME(a.MIMEType) {
			sb.WriteString(fmt.Sprintf("### %s (binary, %d bytes)\n\n", a.OrigName, a.Size))
			continue
		}
		data, err := os.ReadFile(uploads.AbsPath(a))
		if err != nil {
			continue
		}
		content := string(data)
		remaining := maxTotalChars - totalChars
		if remaining <= 0 {
			break
		}
		if len(content) > maxAttachmentChars {
			content = content[:maxAttachmentChars] + "\n[...truncated]"
		}
		if len(content) > remaining {
			content = content[:remaining] + "\n[...truncated]"
		}
		totalChars += len(content)
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", a.OrigName, content))
	}
	return sb.String()
}

// SkeletonJSON serializes the active skeleton config + MCP skeleton steps to JSON.
// Emits an ordered phases array with hints so the LLM knows what each phase means.
func SkeletonJSON(sk config.SkeletonConfig, mcpServers map[string]config.MCPServer, phaseHints map[string]string) string {
	type phase struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
		Hint    string `json:"hint,omitempty"`
	}
	type mcpPhase struct {
		Label    string `json:"label"`
		Prompt   string `json:"prompt"`
		ReadOnly bool   `json:"read_only"`
	}
	type skeleton struct {
		Phases   []phase    `json:"phases"`
		MCPSteps []mcpPhase `json:"mcp_steps,omitempty"`
	}

	orderedPhases := []struct {
		id      string
		enabled bool
	}{
		{"doc_setup", sk.DocSetup},
		{"web_search", sk.WebSearch},
		{"build_verify", sk.BuildVerify},
		{"test", sk.Test},
		{"security_review", sk.SecurityReview},
		{"pre_commit", sk.PreCommit},
		{"commit", sk.Commit},
		{"push", sk.Push},
	}

	s := skeleton{}
	for _, op := range orderedPhases {
		s.Phases = append(s.Phases, phase{
			ID:      op.id,
			Enabled: op.enabled,
			Hint:    phaseHints[op.id],
		})
	}
	for _, mcp := range mcpServers {
		if mcp.SkeletonStep != nil && mcp.Enabled {
			s.MCPSteps = append(s.MCPSteps, mcpPhase{
				Label:    mcp.SkeletonStep.Label,
				Prompt:   mcp.SkeletonStep.Prompt,
				ReadOnly: mcp.SkeletonStep.ReadOnly,
			})
		}
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

// BuildPlanPrompt builds the full prompt for plan generation.
func BuildPlanPrompt(t queue.Task, skeletonBlock, projectsDir string) string {
	attachmentBlock := buildAttachmentBlock(t.Attachments)
	contextSection := ""
	if attachmentBlock != "" {
		contextSection = "\nCONTEXT FROM ATTACHMENTS:\n" + attachmentBlock + "\n"
	}

	const tpl = `You are a plan generator for project %s/%s.

## Execution sequence

1. Call Skill tool with skill='bmad:core:workflows:party-mode' — wait for it to complete before proceeding.
2. Follow the skeleton phases below — each enabled phase becomes a step. Respect ordering hints in each phase.
3. Emit the plan as your final text message.

## Development methodology

When generating implementation steps, ALWAYS follow this approach:

1. FRONTEND FIRST: Build the UI/frontend with mock data. Use MOCK_ prefix for all mock variables and dedicated mock files. Verify visually with Chrome DevTools (take screenshot, check DOM, verify no console errors).
2. BACKEND IMPLEMENTATION: Implement real backend logic, API endpoints, data connections. Replace mock imports with real implementations.
3. MOCK CLEANUP: Remove ALL mock data before proceeding to build/test phases. Grep for MOCK_, mockData, mock_, fake_, dummy_ — ZERO matches allowed in production code (test files excluded). Take final screenshot confirming UI works identically without mocks.

This is not optional. Every task that involves UI/frontend work MUST follow this sequence.

## Task

%s

%s## Skeleton phases

%s

## ReadOnly rules

- Action phases (build_verify, test, pre_commit, commit, push) MUST be ReadOnly: false — they execute commands.
- Only investigation phases (doc_setup, web_search) may be ReadOnly: true.
- The push step MUST run git push origin <branch> via Bash. It is NOT guidance.

## Output format

CRITICAL: No leading tabs or spaces on any line. Use proper markdown with blank lines between sections. Use bullet lists for step instructions.

# Plan: [concise title]

## Analysis

[2-3 sentences summarizing investigation findings]

## Steps

### Step N: [title]

Agent: [bmad agent id assigned by party-mode]
ReadOnly: true|false

- [instruction as bullet point]
- [instruction as bullet point]

Verify: [success criteria]

## Constraints

- [constraint as bullet point]

5-12 steps total. Do not create files. Final message must be the plan text.`

	return fmt.Sprintf(tpl, projectsDir, t.Project, t.Description, contextSection, skeletonBlock)
}

// PlanToolMessage creates a human-readable log message from a tool_use event.
func PlanToolMessage(name string, input map[string]any) string {
	str := func(key string) string {
		if v, ok := input[key]; ok {
			if s, ok := v.(string); ok {
				if len(s) > 80 {
					return s[:80] + "..."
				}
				return s
			}
		}
		return ""
	}
	switch name {
	case "Read":
		if p := str("file_path"); p != "" {
			return "Reading: " + filepath.Base(p)
		}
	case "Glob":
		if p := str("pattern"); p != "" {
			return "Scanning: " + p
		}
	case "Grep":
		if p := str("pattern"); p != "" {
			return "Searching: " + p
		}
	case "Bash":
		if c := str("command"); c != "" {
			return "Running: " + c
		}
	case "WebSearch":
		if q := str("query"); q != "" {
			return "Researching: " + q
		}
	case "WebFetch":
		if u := str("url"); u != "" {
			return "Fetching: " + u
		}
	case "Skill":
		if s := str("skill"); s != "" {
			return "Loading BMAD: " + s
		}
	case "Task":
		if d := str("description"); d != "" {
			return "Delegating: " + d
		}
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.SplitN(name, "__", 3)
		if len(parts) >= 3 {
			return parts[1] + ": " + strings.ReplaceAll(parts[2], "-", " ")
		}
	}
	return "Planning: " + name
}

// GeneratePlan runs the backend to generate a plan synchronously and saves it.
// logFn is called with descriptive messages as planning progresses (may be nil).
func GeneratePlan(t queue.Task, sk config.SkeletonConfig, cfg config.Config, logFn func(string)) (plan.Plan, error) {
	// Ensure .bmad symlink exists so BMAD workflows can resolve @.bmad/ paths
	projectDir := filepath.Join(cfg.ProjectsDir, t.Project)
	projectinit.EnsureBMADLink(projectDir)

	// BMAD must be available — party-mode handles agent assignments
	bmadDir := filepath.Join(projectDir, ".bmad")
	if _, err := os.Stat(bmadDir); err != nil {
		return plan.Plan{}, fmt.Errorf("BMAD not available at %s — run onboarding first", bmadDir)
	}

	skeletonBlock := SkeletonJSON(sk, cfg.MCPServers, cfg.PhaseHints)
	prompt := BuildPlanPrompt(t, skeletonBlock, cfg.ProjectsDir)

	env := backend.FilterEnv(os.Environ(), "CLAUDECODE")
	gitName := engine.GenerateName()
	gitEmail := engine.GenerateEmail(gitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+gitName,
		"GIT_AUTHOR_EMAIL="+gitEmail,
		"GIT_COMMITTER_NAME="+gitName,
		"GIT_COMMITTER_EMAIL="+gitEmail,
	)

	b := backend.Resolve(config.BackendFor(cfg, t.Project))

	req := backend.SpawnRequest{
		Prompt:     prompt,
		ProjectDir: projectDir,
		WorkDir:    projectDir,
		Model:      b.ResolveModel(cfg.Spawn.Model, "plan"),
		MaxTurns:   cfg.Spawn.PlanMaxTurns,
		Env:        env,
		Phase:      "plan",
		DisallowedTools: []string{
			"Edit", "Write", "NotebookEdit", "Bash",
			"ExitPlanMode", "EnterPlanMode", "TodoWrite",
			"Task", "AskUserQuestion",
		},
		// No MCPConfig — plan gen intentionally omits MCPs
	}

	timeout := time.Duration(cfg.Spawn.PlanTimeoutMin) * time.Minute
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	events := make(chan backend.Event, 64)
	go func() {
		b.Execute(ctx, req, events)
	}()

	// Heartbeat: periodic progress during stream gaps
	planStart := time.Now()
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ticker.C:
				if logFn != nil {
					elapsed := time.Since(planStart).Round(time.Second)
					logFn(fmt.Sprintf("Still planning... (%s elapsed)", elapsed))
				}
			}
		}
	}()

	var planResult string
	var planText strings.Builder
	planCaptured := false

	for ev := range events {
		switch ev.Type {
		case "assistant":
			if ev.ToolName != "" && logFn != nil {
				logFn(PlanToolMessage(ev.ToolName, ev.ToolInput))
			}
			if ev.Text != "" {
				planText.WriteString(ev.Text)
				txt := planText.String()
				if strings.Contains(txt, "# Plan:") && strings.Contains(txt, "## Steps") && strings.Contains(txt, "## Constraints") {
					planResult = txt
					planCaptured = true
					cancel() // early kill — Execute will terminate the process via context
				}
			}
		case "result":
			if !planCaptured && ev.Result != "" {
				planResult = ev.Result
			}
		}
	}
	close(heartbeatDone)

	if ctx.Err() == context.DeadlineExceeded {
		return plan.Plan{}, fmt.Errorf("plan generation timed out after %v", timeout)
	}

	if planResult == "" {
		return plan.Plan{}, fmt.Errorf("plan generation returned empty result")
	}

	if err := plan.SavePlan(t.ID, planResult); err != nil {
		return plan.Plan{}, fmt.Errorf("saving plan: %w", err)
	}
	if err := queue.SetPlanFile(t.ID, plan.PlanPath(t.ID)); err != nil {
		return plan.Plan{}, fmt.Errorf("setting plan file: %w", err)
	}
	p, err := plan.ParsePlan(plan.PlanPath(t.ID))
	if err != nil {
		return plan.Plan{}, fmt.Errorf("parsing generated plan: %w", err)
	}
	return p, nil
}
