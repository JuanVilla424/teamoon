package plangen

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

// BuildSkeletonPrompt builds the skeleton step instructions from a SkeletonConfig.
func BuildSkeletonPrompt(sk config.SkeletonConfig) string {
	var sb strings.Builder
	sb.WriteString("\n\nSKELETON — Your plan MUST follow this structure. The Investigate step is ALWAYS present.\n")
	sb.WriteString("Generate implementation steps (Step 3..N-x) based on the task, then append the enabled tail steps.\n\n")

	sb.WriteString("Step 1: Investigate codebase [ALWAYS ON] [ReadOnly]\n")
	sb.WriteString("ReadOnly: true\n")
	sb.WriteString("- Read CLAUDE.md, MEMORY.md, CONTEXT.md, README.md, INSTALL.md, VERSIONING.md, CONTRIBUTING.md\n")
	sb.WriteString("- Read all relevant source files\n")
	sb.WriteString("- Understand architecture, patterns, conventions\n")
	if sk.WebSearch {
		sb.WriteString("- Search the web if needed for context\n")
	}
	sb.WriteString("- Summarize findings\n\n")

	if sk.Context7Lookup {
		sb.WriteString("Step 2: Look up library documentation [ReadOnly]\n")
		sb.WriteString("ReadOnly: true\n")
		sb.WriteString("- Use resolve-library-id then query-docs for each relevant library\n")
		sb.WriteString("- Note relevant APIs and patterns\n\n")
	}

	sb.WriteString("Step 3..N-x: Implementation steps [YOU GENERATE THESE]\n")
	sb.WriteString("- Actual code changes, file edits specific to the task\n")
	sb.WriteString("- Number of steps varies per task\n\n")

	if sk.BuildVerify {
		sb.WriteString("Tail step: Build and verify\n")
		sb.WriteString("- Compile/build the project (make build, go build, npx vite build, etc.)\n")
		sb.WriteString("- If no build system exists, set one up (Makefile, package.json scripts, etc.)\n")
		sb.WriteString("- Verify no compilation errors or warnings\n")
		sb.WriteString("- If applicable, verify the service starts correctly\n\n")
	}

	if sk.Test {
		sb.WriteString("Tail step: Test\n")
		sb.WriteString("- If no test infrastructure exists, create it (go test, pytest, vitest, etc.)\n")
		sb.WriteString("- Run existing test suite for affected packages\n")
		sb.WriteString("- Write new unit/integration tests covering the implementation changes\n")
		sb.WriteString("- Run the full test suite and fix any failures\n")
		sb.WriteString("- All tests must pass before proceeding\n\n")
	}

	if sk.PreCommit {
		sb.WriteString("Tail step: Pre-commit\n")
		sb.WriteString("- If no pre-commit hooks exist, set them up\n")
		sb.WriteString("- Run all pre-commit checks on changed files\n")
		sb.WriteString("- Fix any issues found (formatting, linting, validation)\n")
		sb.WriteString("- Re-run until all checks pass\n\n")
	}

	if sk.Commit {
		sb.WriteString("Tail step: Commit\n")
		sb.WriteString("- Update CHANGELOG.md: add entries under [Unreleased] in the appropriate section (Added/Changed/Fixed/Removed)\n")
		sb.WriteString("- Stage all changed files (specific files, not git add -A)\n")
		sb.WriteString("- Commit with format: type(core): description in lowercase\n")
		sb.WriteString("- Check VERSIONING.md for valid commit types and optional versioning keywords\n")
		sb.WriteString("- Check CONTRIBUTING.md if contributing to an external repo\n")
		sb.WriteString("- NO emojis/icons in commit message\n")
		sb.WriteString("- Single commit grouping all changes\n")
		sb.WriteString("- If pre-commit hooks fail on commit, fix issues and retry\n\n")
	}

	if sk.Push {
		sb.WriteString("Tail step: Push\n")
		sb.WriteString("- Push to current remote branch\n")
		sb.WriteString("- Verify push succeeds\n\n")
	}

	return sb.String()
}

// BuildPlanPrompt builds the full prompt for plan generation.
func BuildPlanPrompt(t queue.Task, skeletonBlock string) string {
	return fmt.Sprintf(
		"You may read files to understand the codebase, then create a plan.\n"+
			"Project path: /home/cloud-agent/Projects/%s\n\n"+
			"TASK: %s\n\n"+
			"INSTRUCTIONS:\n"+
			"1. FIRST read CLAUDE.md in the project root for project-specific guidelines\n"+
			"2. Read the relevant source files to understand the current code\n"+
			"3. Then output your final plan as a TEXT MESSAGE (not a tool call)\n\n"+
			"%s\n\n"+
			"Your FINAL message MUST be the plan in this markdown format:\n\n"+
			"# Plan: %s\n\n"+
			"## Analysis\n[what you found in the codebase]\n\n"+
			"## Steps\n\n### Step 1: [title]\nAgent: [agent-id]\nReadOnly: true/false\n[instructions with specific files and changes]\nVerify: [verification]\n\n"+
			"### Step 2: [title]\nAgent: [agent-id]\nReadOnly: true/false\n[instructions]\nVerify: [verification]\n\n"+
			"(5-12 steps total, use as many agents as needed)\n\n"+
			"## Constraints\n- [constraints]\n\n"+
			"AGENT ASSIGNMENT (MANDATORY for every step):\n"+
			"Each step MUST have an 'Agent:' line. Available BMAD agents:\n"+
			"- analyst (Mary - requirements analysis, research, data gathering)\n"+
			"- pm (John - product strategy, feature scoping, prioritization)\n"+
			"- architect (Winston - system design, architecture, technical decisions)\n"+
			"- ux-designer (Sally - UX/UI design, user flows, accessibility)\n"+
			"- dev (Amelia - implementation, coding, file changes)\n"+
			"- tea (Murat - testing, QA, test architecture, validation)\n"+
			"- sm (Bob - task breakdown, sprint management, story prep)\n"+
			"- tech-writer (Paige - documentation, README, API docs)\n"+
			"- cloud (Warren - cloud infrastructure, deployment, CI/CD)\n"+
			"- quick-flow-solo-dev (Barry - rapid prototyping, full-stack quick tasks)\n"+
			"- brainstorming-coach (Carson - ideation, creative exploration)\n"+
			"- creative-problem-solver (Dr. Quinn - root cause analysis, complex debugging)\n"+
			"- design-thinking-coach (Maya - user-centered design process)\n"+
			"- innovation-strategist (Victor - business model, strategic decisions)\n\n"+
			"Distribute steps across ALL relevant agents. A typical plan should involve:\n"+
			"1. Analysis/research agent first (analyst or pm)\n"+
			"2. Architecture/design agents (architect, ux-designer)\n"+
			"3. Implementation agents (dev, quick-flow-solo-dev)\n"+
			"4. Testing agent (tea)\n"+
			"5. Documentation/deployment agents (tech-writer, cloud)\n\n"+
			"ReadOnly ANNOTATION: Add 'ReadOnly: true' to steps that only read/research (like Investigate and Library Lookup).\n"+
			"Add 'ReadOnly: false' or omit for steps that modify files.\n\n"+
			"CRITICAL: Your very last message MUST be the markdown plan text, NOT a tool call.\n\n"+
			"PROHIBITIONS:\n"+
			"- NEVER invoke /bmad:core:workflows:party-mode or any /bmad slash command\n"+
			"- NEVER use EnterPlanMode — you ARE generating the plan already\n"+
			"- NEVER create .md files (SUMMARY.md, ANALYSIS.md, etc.) — output the plan as a text message only\n"+
			"- NEVER include steps that say 'invoke party mode' or 'load workflow' — the autopilot system handles orchestration\n"+
			"- Be concise and direct. No preamble.",
		t.Project, t.Description, skeletonBlock, t.Description,
	)
}

// GeneratePlan runs claude to generate a plan synchronously and saves it.
func GeneratePlan(t queue.Task, sk config.SkeletonConfig, cfg config.Config) (plan.Plan, error) {
	skeletonBlock := BuildSkeletonPrompt(sk)
	prompt := BuildPlanPrompt(t, skeletonBlock)

	env := filterEnv(os.Environ(), "CLAUDECODE")
	gitName := engine.GenerateName()
	gitEmail := engine.GenerateEmail(gitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+gitName,
		"GIT_AUTHOR_EMAIL="+gitEmail,
		"GIT_COMMITTER_NAME="+gitName,
		"GIT_COMMITTER_EMAIL="+gitEmail,
	)

	planCfg := cfg
	planCfg.Spawn.MaxTurns = 50
	args, cleanup := engine.BuildSpawnArgs(planCfg, prompt, nil)
	if cleanup != nil {
		defer cleanup()
	}
	// Override stream-json to json for plan gen
	for i, a := range args {
		if a == "--output-format" && i+1 < len(args) {
			args[i+1] = "json"
		}
		if a == "--verbose" {
			args = append(args[:i], args[i+1:]...)
			break
		}
	}
	cmd := exec.Command("claude", args...)
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		return plan.Plan{}, fmt.Errorf("plan generation failed: %w", err)
	}

	var result struct {
		Result  string `json:"result"`
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	var planText string
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		planText = strings.TrimSpace(string(out))
		if planText == "" {
			return plan.Plan{}, fmt.Errorf("plan generation returned empty output")
		}
	} else if result.Result == "" {
		return plan.Plan{}, fmt.Errorf("plan result empty (subtype: %s)", result.Subtype)
	} else {
		planText = result.Result
	}

	if err := plan.SavePlan(t.ID, planText); err != nil {
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
