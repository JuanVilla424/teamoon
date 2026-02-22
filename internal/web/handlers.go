package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/chat"
	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/projectinit"
	"github.com/JuanVilla424/teamoon/internal/projects"
	"github.com/JuanVilla424/teamoon/internal/queue"
	"github.com/JuanVilla424/teamoon/internal/templates"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleTaskAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Project     string `json:"project"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	t, err := queue.Add(req.Project, req.Description, req.Priority)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, t)
}

func (s *Server) handleTaskDone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if s.store.engineMgr.IsRunning(req.ID) {
		s.store.engineMgr.Stop(req.ID)
	}
	if err := queue.MarkDone(req.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTaskArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if s.store.engineMgr.IsRunning(req.ID) {
		s.store.engineMgr.Stop(req.ID)
	}
	if err := queue.Archive(req.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTaskReplan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if s.store.engineMgr.IsRunning(req.ID) {
		s.store.engineMgr.Stop(req.ID)
	}
	s.clearGenerating(req.ID)
	os.Remove(plan.PlanPath(req.ID))
	if err := queue.ResetPlan(req.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTaskStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if s.store.engineMgr.IsRunning(req.ID) {
		s.store.engineMgr.Stop(req.ID)
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTaskAutopilot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID  int   `json:"id"`
		Run *bool `json:"run,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	tasks, _ := queue.ListActive()
	var found queue.Task
	for _, t := range tasks {
		if t.ID == req.ID {
			found = t
			break
		}
	}
	if found.ID == 0 {
		writeErr(w, 404, "task not found")
		return
	}

	state := queue.EffectiveState(found)

	// If currently generating, block duplicate calls
	if state == queue.StatePending && s.isGenerating(found.ID) {
		writeJSON(w, map[string]string{"status": "already_generating"})
		return
	}

	switch state {
	case queue.StatePending:
		autoRun := req.Run == nil || *req.Run
		s.setGenerating(found.ID)
		s.refreshAndBroadcast()
		go s.generatePlanAsync(found, autoRun)
		writeJSON(w, map[string]string{"status": "generating"})

	case queue.StatePlanned:
		p, err := plan.ParsePlan(plan.PlanPath(found.ID))
		if err != nil {
			writeErr(w, 500, "plan parse error: "+err.Error())
			return
		}
		queue.UpdateState(found.ID, queue.StateRunning)
		s.store.engineMgr.Start(found, p, s.cfg, s.webSend(found.ID))
		s.refreshAndBroadcast()
		writeJSON(w, map[string]string{"status": "running"})

	case queue.StateRunning:
		s.store.engineMgr.Stop(found.ID)
		s.refreshAndBroadcast()
		writeJSON(w, map[string]string{"status": "stopped"})

	case queue.StateBlocked:
		p, err := plan.ParsePlan(plan.PlanPath(found.ID))
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		queue.UpdateState(found.ID, queue.StateRunning)
		s.store.engineMgr.Start(found, p, s.cfg, s.webSend(found.ID))
		s.refreshAndBroadcast()
		writeJSON(w, map[string]string{"status": "resumed"})

	default:
		writeJSON(w, map[string]string{"status": "no_action"})
	}
}

// In-memory generating tracker
var (
	generatingMu sync.Mutex
	generatingSet = map[int]bool{}
)

func (s *Server) isGenerating(id int) bool {
	generatingMu.Lock()
	defer generatingMu.Unlock()
	return generatingSet[id]
}

func (s *Server) setGenerating(id int) {
	generatingMu.Lock()
	generatingSet[id] = true
	generatingMu.Unlock()
}

func (s *Server) clearGenerating(id int) {
	generatingMu.Lock()
	delete(generatingSet, id)
	generatingMu.Unlock()
}

func (s *Server) webSend(taskID int) func(tea.Msg) {
	return func(msg tea.Msg) {
		switch m := msg.(type) {
		case engine.LogMsg:
			s.store.logBuf.Add(m.Entry)
			s.scheduleRefresh() // debounced — avoid flooding SSE on rapid log output
			return
		case engine.TaskStateMsg:
			s.store.logBuf.Add(logs.LogEntry{
				Time:    time.Now(),
				TaskID:  m.TaskID,
				Message: "State: " + string(m.State),
				Level:   logs.LevelInfo,
			})
		case engine.PlanGeneratedMsg:
			if m.Err == nil && m.Content != "" {
				plan.SavePlan(m.TaskID, m.Content)
				queue.SetPlanFile(m.TaskID, plan.PlanPath(m.TaskID))
			}
		}
		s.refreshAndBroadcast() // immediate for state changes
	}
}

func (s *Server) handleTaskPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	var id int
	fmt.Sscanf(r.URL.Query().Get("id"), "%d", &id)
	if !plan.PlanExists(id) {
		writeErr(w, 404, "no plan")
		return
	}
	content, err := os.ReadFile(plan.PlanPath(id))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]string{"content": string(content)})
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	var id int
	fmt.Sscanf(r.URL.Query().Get("id"), "%d", &id)
	entries := logs.ReadTaskLog(id)
	logJSON := make([]LogEntryJSON, len(entries))
	for i, e := range entries {
		lvl := "info"
		switch e.Level {
		case logs.LevelSuccess:
			lvl = "success"
		case logs.LevelWarn:
			lvl = "warn"
		case logs.LevelError:
			lvl = "error"
		}
		logJSON[i] = LogEntryJSON{
			Time:    e.Time,
			TaskID:  e.TaskID,
			Project: e.Project,
			Message: e.Message,
			Level:   lvl,
			Agent:   e.Agent,
		}
	}
	writeJSON(w, map[string]any{"task_id": id, "logs": logJSON})
}

func (s *Server) handleProjectPRs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	repo := r.URL.Query().Get("repo")
	prs, err := projects.FetchPRs(repo)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	depBot := projects.FilterDependabot(prs)
	writeJSON(w, map[string]any{"prs": prs, "dependabot": depBot})
}

func (s *Server) handleMergeDependabot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Repo string `json:"repo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	prs, err := projects.FetchPRs(req.Repo)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	depBot := projects.FilterDependabot(prs)
	merged, failed := 0, 0
	for _, pr := range depBot {
		if err := projects.MergePR(req.Repo, pr.Number); err != nil {
			failed++
		} else {
			merged++
		}
	}
	writeJSON(w, map[string]int{"merged": merged, "failed": failed})
}

func (s *Server) handleProjectPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	out, err := projects.GitPull(req.Path)
	if err != nil {
		writeErr(w, 500, out)
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]string{"output": out})
}

func (s *Server) handleProjectGitInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	projectType := projects.DetectProjectType(req.Path)
	out, backupDir, createdNew, err := projects.GitInitRepo(req.Path, req.Name)
	if err != nil {
		writeErr(w, 500, out+"\n"+err.Error())
		return
	}

	taskCreated := false
	if createdNew {
		desc := buildInitTaskDescription(req.Name, projectType, backupDir)
		t, addErr := queue.Add(req.Name, desc, "high")
		if addErr == nil {
			queue.UpdateAssignee(t.ID, "agent")
			taskCreated = true
		}
	}

	s.refreshAndBroadcast()
	writeJSON(w, map[string]any{"output": out, "task_created": taskCreated})
}

func buildInitTaskDescription(name, projectType, backupDir string) string {
	manifest := "package.json"
	switch projectType {
	case "python":
		manifest = "pyproject.toml"
	case "go":
		manifest = "go.mod"
	}

	typeWorkflow := "node.yml"
	switch projectType {
	case "python":
		typeWorkflow = "python.yml"
	case "go":
		typeWorkflow = "go.yml"
	}

	desc := fmt.Sprintf(`Post-INIT setup for %s (%s project):

BACKUP: Original project files are at %s — restore them first (copy all files from backup into project dir, overwriting template versions). Do NOT copy .git or node_modules from backup.

1. Remove template junk: delete backend/, frontend/, cazira/, external-resources/, CNAME, .gitmodules directories. Keep scripts/ (submodule). If project is NOT python, also remove root pyproject.toml, src/. Rename template's requirements.txt to requirements.dev.txt, then restore the project's original requirements.txt from backup (if it exists)
2. Modify .bumpversion.cfg: set current_version to 1.0.2, remove [bumpversion:file:backend/pyproject.toml] and [bumpversion:file:frontend/package.json] sections, keep only [bumpversion:file:%s]
3. Update %s version to 1.0.2
4. Reset CHANGELOG.md to initial template (just header and empty sections)
5. Trim .github/workflows/: keep only ci.yml, %s, version-controller.yml, release-controller.yml, stale.yml, greetings.yml — delete all others
6. Replace ALL references to "github-cicd-template" or "GitHub CI/CD Template" in every file (README.md, SECURITY.md, CODE_OF_CONDUCT.md, CONTRIBUTING.md, INSTALL.md, VERSIONING.md, CHANGELOG.md, LICENSE, etc.) with the actual project name. Check every .md file and config file for stale template references
7. Generate a professional README.md following the github-cicd-template pattern: H1 with emoji + project name, shields.io badges row (language, version, build status, status, license), project description paragraph, Table of Contents with anchor links, Features section with bullet points, Getting Started with Prerequisites + Installation + Environment Setup + Pre-Commit Hooks subsections, Usage section, Contributing section referencing CONTRIBUTING.md, License and Contact sections matching the style of the other .md files already in the repo
8. Ensure .gitignore includes: CLAUDE.md, MEMORY.md, CONTEXT.md, .env, .env.local, *.pem, certs/, secrets/, keys/, venv/, node_modules/
9. Configure pre-commit: pip install pre-commit && pre-commit install && pre-commit install --hook-type pre-push
10. If python: create venv (python3 -m venv venv), install requirements if requirements.txt exists
11. git checkout -b dev (if not already on dev)
12. git add . && git commit -m "feat(core): initial project scaffold"
13. git push -u origin dev

PROHIBIDO: git push --force, git reset --hard, --no-verify en cualquier comando.

usar /bmad:core:workflows:party-mode`, name, projectType, backupDir, manifest, manifest, typeWorkflow)

	return desc
}

func buildSkeletonPrompt(sk config.SkeletonConfig) string {
	var sb strings.Builder
	sb.WriteString("\n\nSKELETON — Your plan MUST follow this structure. The Investigate step is ALWAYS present.\n")
	sb.WriteString("Generate implementation steps (Step 3..N-x) based on the task, then append the enabled tail steps.\n\n")

	sb.WriteString("Step 1: Investigate codebase [ALWAYS ON] [ReadOnly]\n")
	sb.WriteString("ReadOnly: true\n")
	sb.WriteString("- Read CLAUDE.md, MEMORY.md, CONTEXT.md\n")
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
		sb.WriteString("- Stage all changed files (specific files, not git add -A)\n")
		sb.WriteString("- Commit with format: type(core): description in lowercase\n")
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

func (s *Server) generatePlanAsync(t queue.Task, autoRun bool) {
	skeletonBlock := buildSkeletonPrompt(s.cfg.Skeleton)

	prompt := fmt.Sprintf(
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

	s.store.logBuf.Add(logs.LogEntry{
		Time: time.Now(), TaskID: t.ID, Project: t.Project,
		Message: "Generating plan...", Level: logs.LevelInfo,
	})
	s.refreshAndBroadcast()

	env := filterEnvWeb(os.Environ(), "CLAUDECODE")
	gitName := engine.GenerateName()
	gitEmail := engine.GenerateEmail(gitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+gitName,
		"GIT_AUTHOR_EMAIL="+gitEmail,
		"GIT_COMMITTER_NAME="+gitName,
		"GIT_COMMITTER_EMAIL="+gitEmail,
	)
	// Use BuildSpawnArgs but override output-format to json for plan generation
	planCfg := s.cfg
	planCfg.Spawn.MaxTurns = 50 // plan gen needs more turns
	args, spawnCleanup := engine.BuildSpawnArgs(planCfg, prompt, nil)
	if spawnCleanup != nil {
		defer spawnCleanup()
	}
	// Override stream-json to json for plan gen (we need full output, not streaming)
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
		s.store.logBuf.Add(logs.LogEntry{
			Time: time.Now(), TaskID: t.ID, Project: t.Project,
			Message: "Plan generation failed: " + err.Error(), Level: logs.LevelError,
		})
		s.clearGenerating(t.ID)
		s.refreshAndBroadcast()
		return
	}

	var result struct {
		Result  string `json:"result"`
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		// Try as plain text fallback
		planText := strings.TrimSpace(string(out))
		if planText == "" {
			s.store.logBuf.Add(logs.LogEntry{
				Time: time.Now(), TaskID: t.ID, Project: t.Project,
				Message: "Plan generation returned empty output", Level: logs.LevelError,
			})
			s.clearGenerating(t.ID)
			s.refreshAndBroadcast()
			return
		}
		plan.SavePlan(t.ID, planText)
	} else if result.Result == "" {
		preview := string(out)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		s.store.logBuf.Add(logs.LogEntry{
			Time: time.Now(), TaskID: t.ID, Project: t.Project,
			Message: "Plan result empty (subtype: " + result.Subtype + ") | raw: " + preview, Level: logs.LevelError,
		})
		s.clearGenerating(t.ID)
		s.refreshAndBroadcast()
		return
	} else {
		plan.SavePlan(t.ID, result.Result)
	}
	queue.SetPlanFile(t.ID, plan.PlanPath(t.ID))
	s.store.logBuf.Add(logs.LogEntry{
		Time: time.Now(), TaskID: t.ID, Project: t.Project,
		Message: "Plan generated", Level: logs.LevelSuccess,
	})
	s.clearGenerating(t.ID)
	s.refreshAndBroadcast()

	if autoRun {
		p, err := plan.ParsePlan(plan.PlanPath(t.ID))
		if err != nil {
			s.store.logBuf.Add(logs.LogEntry{
				Time: time.Now(), TaskID: t.ID, Project: t.Project,
				Message: "Auto-run failed: " + err.Error(), Level: logs.LevelError,
			})
			s.clearGenerating(t.ID)
			s.refreshAndBroadcast()
			return
		}
		queue.UpdateState(t.ID, queue.StateRunning)
		s.store.engineMgr.Start(t, p, s.cfg, s.webSend(t.ID))
		s.store.logBuf.Add(logs.LogEntry{
			Time: time.Now(), TaskID: t.ID, Project: t.Project,
			Message: "Auto-running task", Level: logs.LevelInfo,
		})
		s.refreshAndBroadcast()
	}
}

func (s *Server) handleTemplateList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	list, err := templates.List()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"templates": list})
}

func (s *Server) handleTemplateAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Name == "" || req.Content == "" {
		writeErr(w, 400, "name and content required")
		return
	}
	t, err := templates.Add(req.Name, req.Content)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, t)
}

func (s *Server) handleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := templates.Delete(req.ID); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	t, err := templates.Update(req.ID, req.Name, req.Content)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, t)
}

func filterEnvWeb(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// --- Chat handlers ---

func (s *Server) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	msgs, err := chat.LoadHistory()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"messages": msgs})
}

func (s *Server) handleChatClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	if err := chat.ClearHistory(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleChatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Message string `json:"message"`
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Message == "" {
		writeErr(w, 400, "message required")
		return
	}

	// Save user message
	chat.AppendMessage(chat.Message{
		Role:      "user",
		Content:   req.Message,
		Project:   req.Project,
		Timestamp: time.Now(),
	})

	// Build prompt with recent context
	recent := chat.RecentContext(10)
	var promptBuf strings.Builder
	promptBuf.WriteString("You are a helpful AI assistant for software engineering tasks.\n\n")
	promptBuf.WriteString("You can create tasks in the project queue by including directives in your response.\n")
	promptBuf.WriteString("Format: [TASK_CREATE]{\"description\":\"task desc\",\"priority\":\"med\",\"assignee\":\"human\"}[/TASK_CREATE]\n")
	promptBuf.WriteString("- priority: \"high\", \"med\", or \"low\"\n")
	promptBuf.WriteString("- assignee: \"human\" (for the user to do) or \"agent\" (for AI autopilot)\n")
	promptBuf.WriteString("Use this when the user asks you to create tasks, break work into subtasks, or set up prerequisites.\n")
	promptBuf.WriteString("Each directive creates one task. You can include multiple directives in one response.\n")
	promptBuf.WriteString("Always explain what tasks you are creating in your text before or after the directives.\n\n")
	if req.Project != "" {
		home, _ := os.UserHomeDir()
		promptBuf.WriteString(fmt.Sprintf("Working on project: %s (path: %s)\n\n",
			req.Project, fmt.Sprintf("%s/Projects/%s", home, req.Project)))
	}
	if len(recent) > 1 {
		promptBuf.WriteString("Conversation history:\n")
		for _, m := range recent[:len(recent)-1] {
			promptBuf.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
		}
		promptBuf.WriteString("\n")
	}
	promptBuf.WriteString(req.Message)

	// Spawn claude with stream-json
	home, _ := os.UserHomeDir()
	projectPath := home
	if req.Project != "" {
		pp := fmt.Sprintf("%s/Projects/%s", home, req.Project)
		if _, err := os.Stat(pp); err == nil {
			projectPath = pp
		}
	}

	env := filterEnvWeb(os.Environ(), "CLAUDECODE")
	chatGitName := engine.GenerateName()
	chatGitEmail := engine.GenerateEmail(chatGitName)
	env = append(env,
		"GIT_AUTHOR_NAME="+chatGitName,
		"GIT_AUTHOR_EMAIL="+chatGitEmail,
		"GIT_COMMITTER_NAME="+chatGitName,
		"GIT_COMMITTER_EMAIL="+chatGitEmail,
	)
	chatCfg := s.cfg
	chatCfg.Spawn.MaxTurns = 10 // chat uses fewer turns
	chatArgs, chatCleanup := engine.BuildSpawnArgs(chatCfg, promptBuf.String(), nil)
	if chatCleanup != nil {
		defer chatCleanup()
	}
	cmd := exec.Command("claude", chatArgs...)
	cmd.Env = env
	cmd.Dir = projectPath

	devNull, _ := os.Open(os.DevNull)
	if devNull != nil {
		cmd.Stdin = devNull
		defer devNull.Close()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	// Stream SSE response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming not supported")
		return
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	type streamEvt struct {
		Type    string `json:"type"`
		Message *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"content"`
		} `json:"message,omitempty"`
		Result string `json:"result,omitempty"`
	}

	for scanner.Scan() {
		line := scanner.Text()
		var evt streamEvt
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		switch evt.Type {
		case "assistant":
			if evt.Message != nil {
				for _, c := range evt.Message.Content {
					if c.Type == "text" && c.Text != "" {
						fullResponse.WriteString(c.Text)
						tokenData, _ := json.Marshal(map[string]string{"token": c.Text})
						fmt.Fprintf(w, "data: %s\n\n", tokenData)
						flusher.Flush()
					}
				}
			}
		case "result":
			if evt.Result != "" {
				fullResponse.Reset()
				fullResponse.WriteString(evt.Result)
			}
			doneData, _ := json.Marshal(map[string]any{"done": true, "result": evt.Result})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
			flusher.Flush()
		}
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		errMsg := stderrBuf.String()
		log.Printf("[chat] claude exited with error: %v", waitErr)
		os.WriteFile("/tmp/teamoon-chat-stderr.log", []byte(errMsg), 0644)
	}

	// Parse task creation directives from response
	responseText := fullResponse.String()
	taskDirectiveRe := regexp.MustCompile(`\[TASK_CREATE\](.*?)\[/TASK_CREATE\]`)
	matches := taskDirectiveRe.FindAllStringSubmatch(responseText, -1)
	var createdTasks []map[string]any

	if len(matches) > 0 {
		type taskDirective struct {
			Description string `json:"description"`
			Priority    string `json:"priority"`
			Assignee    string `json:"assignee"`
		}
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			var td taskDirective
			if err := json.Unmarshal([]byte(m[1]), &td); err != nil {
				continue
			}
			if td.Description == "" {
				continue
			}
			if td.Priority == "" {
				td.Priority = "med"
			}
			if td.Assignee == "" {
				td.Assignee = "human"
			}
			t, err := queue.Add(req.Project, td.Description, td.Priority)
			if err != nil {
				continue
			}
			queue.UpdateAssignee(t.ID, td.Assignee)
			createdTasks = append(createdTasks, map[string]any{
				"id":          t.ID,
				"description": td.Description,
				"priority":    td.Priority,
				"assignee":    td.Assignee,
			})
		}

		// Send tasks_created SSE event
		if len(createdTasks) > 0 {
			tcData, _ := json.Marshal(map[string]any{"tasks_created": createdTasks})
			fmt.Fprintf(w, "data: %s\n\n", tcData)
			flusher.Flush()
			s.refreshAndBroadcast()
		}

		// Strip directives from response before saving
		responseText = taskDirectiveRe.ReplaceAllString(responseText, "")
		responseText = strings.TrimSpace(responseText)
	}

	// Save assistant response
	if responseText != "" {
		chat.AppendMessage(chat.Message{
			Role:      "assistant",
			Content:   responseText,
			Project:   req.Project,
			Timestamp: time.Now(),
		})
	}
}

// --- MCP handlers ---

func (s *Server) handleMCPList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	globalServers := config.ReadGlobalMCPServers()
	usingGlobal := cfg.MCPServers == nil
	custom := cfg.MCPServers
	if custom == nil {
		custom = map[string]config.MCPServer{}
	}
	writeJSON(w, map[string]any{
		"global":       globalServers,
		"custom":       custom,
		"using_global": usingGlobal,
	})
}

func (s *Server) handleMCPToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if cfg.MCPServers == nil {
		writeErr(w, 400, "MCP servers not initialized — call /api/mcp/init first")
		return
	}
	srv, ok := cfg.MCPServers[req.Name]
	if !ok {
		writeErr(w, 404, "server not found: "+req.Name)
		return
	}
	srv.Enabled = req.Enabled
	cfg.MCPServers[req.Name] = srv
	if err := config.Save(cfg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.cfg = cfg
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleMCPInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	config.InitMCPFromGlobal(&cfg)
	if err := config.Save(cfg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.cfg = cfg
	writeJSON(w, map[string]any{
		"ok":      true,
		"servers": cfg.MCPServers,
	})
}

// --- Config handlers ---

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	// Mask password
	pw := ""
	if cfg.WebPassword != "" {
		pw = "***"
	}
	writeJSON(w, map[string]any{
		"projects_dir":        cfg.ProjectsDir,
		"claude_dir":          cfg.ClaudeDir,
		"refresh_interval_sec": cfg.RefreshIntervalSec,
		"budget_monthly":      cfg.BudgetMonthly,
		"context_limit":       cfg.ContextLimit,
		"web_enabled":         cfg.WebEnabled,
		"web_port":            cfg.WebPort,
		"web_password":        pw,
		"webhook_url":         cfg.WebhookURL,
		"spawn_model":         cfg.Spawn.Model,
		"spawn_effort":        cfg.Spawn.Effort,
		"spawn_max_turns":     cfg.Spawn.MaxTurns,
		"skeleton":            cfg.Skeleton,
	})
}

func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ProjectsDir        string                `json:"projects_dir"`
		ClaudeDir          string                `json:"claude_dir"`
		RefreshIntervalSec int                   `json:"refresh_interval_sec"`
		BudgetMonthly      float64               `json:"budget_monthly"`
		ContextLimit       int                   `json:"context_limit"`
		WebEnabled         bool                  `json:"web_enabled"`
		WebPort            int                   `json:"web_port"`
		WebPassword        string                `json:"web_password"`
		WebhookURL         string                `json:"webhook_url"`
		SpawnModel         *string               `json:"spawn_model,omitempty"`
		SpawnEffort        *string               `json:"spawn_effort,omitempty"`
		SpawnMaxTurns      *int                  `json:"spawn_max_turns,omitempty"`
		Skeleton           *config.SkeletonConfig `json:"skeleton,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}

	if req.ProjectsDir != "" {
		cfg.ProjectsDir = req.ProjectsDir
	}
	if req.ClaudeDir != "" {
		cfg.ClaudeDir = req.ClaudeDir
	}
	if req.RefreshIntervalSec > 0 {
		cfg.RefreshIntervalSec = req.RefreshIntervalSec
	}
	cfg.BudgetMonthly = req.BudgetMonthly
	cfg.ContextLimit = req.ContextLimit
	cfg.WebEnabled = req.WebEnabled
	if req.WebPort > 0 {
		cfg.WebPort = req.WebPort
	}
	// Only update password if not masked
	if req.WebPassword != "***" {
		cfg.WebPassword = req.WebPassword
	}
	cfg.WebhookURL = req.WebhookURL
	if req.SpawnModel != nil {
		cfg.Spawn.Model = *req.SpawnModel
	}
	if req.SpawnEffort != nil {
		cfg.Spawn.Effort = *req.SpawnEffort
	}
	if req.SpawnMaxTurns != nil && *req.SpawnMaxTurns > 0 {
		cfg.Spawn.MaxTurns = *req.SpawnMaxTurns
	}
	if req.Skeleton != nil {
		cfg.Skeleton = *req.Skeleton
	}

	if err := config.Save(cfg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.cfg = cfg
	writeJSON(w, map[string]bool{"ok": true})
}

// --- Task Update handler ---

func (s *Server) handleTaskUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Description = strings.TrimSpace(req.Description)
	if req.Description == "" {
		writeErr(w, 400, "description required")
		return
	}
	tasks, _ := queue.ListActive()
	for _, t := range tasks {
		if t.ID == req.ID {
			es := string(queue.EffectiveState(t))
			if es == "running" || es == "done" || es == "archived" {
				writeErr(w, 400, "cannot edit task in "+es+" state")
				return
			}
			break
		}
	}
	if err := queue.UpdateDescription(req.ID, req.Description); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

// --- Canvas: Assignee handler ---

func (s *Server) handleTaskAssignee(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID       int    `json:"id"`
		Assignee string `json:"assignee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := queue.UpdateAssignee(req.ID, req.Assignee); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

// --- Project Init handler ---

func (s *Server) handleProjectInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req projectinit.InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Name == "" {
		writeErr(w, 400, "name required")
		return
	}
	if req.Type == "" {
		req.Type = "python"
	}
	if req.Version == "" {
		req.Version = "1.0.0"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming not supported")
		return
	}

	err := projectinit.RunInit(req, s.cfg.ProjectsDir, func(sr projectinit.StepResult) {
		data, _ := json.Marshal(sr)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	status := "success"
	msg := ""
	if err != nil {
		status = "error"
		msg = err.Error()
	}
	doneData, _ := json.Marshal(map[string]string{"status": status, "message": msg, "done": "true"})
	fmt.Fprintf(w, "data: %s\n\n", doneData)
	flusher.Flush()

	if err == nil {
		s.refreshAndBroadcast()
	}
}

// --- MCP Catalog handlers ---

func (s *Server) handleMCPCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}

	search := r.URL.Query().Get("search")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}

	url := "https://registry.modelcontextprotocol.io/v0.1/servers?limit=" + limit
	if search != "" {
		url += "&search=" + search
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		writeErr(w, 502, "registry request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeErr(w, 502, "reading registry response: "+err.Error())
		return
	}

	// Parse registry response
	var registry struct {
		Servers []struct {
			Server struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Packages    []struct {
					RegistryType         string `json:"registryType"`
					Identifier           string `json:"identifier"`
					EnvironmentVariables []struct {
						Name       string `json:"name"`
						IsRequired bool   `json:"isRequired"`
						IsSecret   bool   `json:"isSecret"`
					} `json:"environmentVariables"`
				} `json:"packages"`
			} `json:"server"`
		} `json:"servers"`
		Metadata struct {
			NextCursor string `json:"nextCursor"`
			Count      int    `json:"count"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &registry); err != nil {
		writeErr(w, 502, "parsing registry response: "+err.Error())
		return
	}

	// Get installed servers for marking
	installed := config.ReadGlobalMCPServers()

	type envVar struct {
		Name       string `json:"name"`
		IsRequired bool   `json:"is_required"`
		IsSecret   bool   `json:"is_secret"`
	}
	type catalogEntry struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Package     string   `json:"package"`
		EnvVars     []envVar `json:"env_vars"`
		Installed   bool     `json:"installed"`
	}

	var results []catalogEntry
	for _, s := range registry.Servers {
		srv := s.Server
		// Find npm package
		for _, pkg := range srv.Packages {
			if pkg.RegistryType != "npm" {
				continue
			}
			entry := catalogEntry{
				Name:        srv.Name,
				Description: srv.Description,
				Package:     pkg.Identifier,
				Installed:   installed[srv.Name].Command != "",
			}
			for _, ev := range pkg.EnvironmentVariables {
				entry.EnvVars = append(entry.EnvVars, envVar{
					Name:       ev.Name,
					IsRequired: ev.IsRequired,
					IsSecret:   ev.IsSecret,
				})
			}
			results = append(results, entry)
			break // only first npm package per server
		}
	}

	writeJSON(w, map[string]any{
		"servers": results,
		"count":   registry.Metadata.Count,
	})
}

func (s *Server) handleMCPInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Name    string            `json:"name"`
		Package string            `json:"package"`
		EnvVars map[string]string `json:"env_vars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Name == "" || req.Package == "" {
		writeErr(w, 400, "name and package required")
		return
	}

	args := []string{"-y", req.Package}
	if err := config.InstallMCPToGlobal(req.Name, "npx", args, req.EnvVars); err != nil {
		writeErr(w, 500, "install failed: "+err.Error())
		return
	}

	// Also add to teamoon config if custom config exists
	cfg, err := config.Load()
	if err == nil && cfg.MCPServers != nil {
		cfg.MCPServers[req.Name] = config.MCPServer{
			Command: "npx",
			Args:    args,
			Enabled: true,
		}
		config.Save(cfg)
		s.cfg = cfg
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// --- Skills handlers ---

func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".agents", "skills")

	type installedSkill struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Path        string `json:"path"`
	}

	var skills []installedSkill
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		writeJSON(w, map[string]any{"skills": skills})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sk := installedSkill{
			Name: entry.Name(),
			Path: filepath.Join(skillsDir, entry.Name()),
		}
		skillMD := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if data, err := os.ReadFile(skillMD); err == nil {
			lines := strings.SplitN(string(data), "\n", 20)
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "description:") {
					sk.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
					sk.Description = strings.Trim(sk.Description, "\"'")
					break
				}
			}
		}
		skills = append(skills, sk)
	}

	writeJSON(w, map[string]any{"skills": skills})
}

func (s *Server) handleSkillsCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}

	search := r.URL.Query().Get("search")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}

	url := "https://skills.sh/api/search?limit=" + limit
	if search != "" {
		url += "&q=" + search
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		writeErr(w, 502, "skills.sh request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeErr(w, 502, "reading skills.sh response: "+err.Error())
		return
	}

	var catalog struct {
		Skills []struct {
			ID       string `json:"id"`
			SkillID  string `json:"skillId"`
			Name     string `json:"name"`
			Installs int    `json:"installs"`
			Source   string `json:"source"`
		} `json:"skills"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		writeErr(w, 502, "parsing skills.sh response: "+err.Error())
		return
	}

	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".agents", "skills")
	installed := map[string]bool{}
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				installed[e.Name()] = true
			}
		}
	}

	type catalogEntry struct {
		ID        string `json:"id"`
		SkillID   string `json:"skill_id"`
		Name      string `json:"name"`
		Source    string `json:"source"`
		Installs  int    `json:"installs"`
		Installed bool   `json:"installed"`
	}

	var results []catalogEntry
	for _, sk := range catalog.Skills {
		entry := catalogEntry{
			ID:        sk.ID,
			SkillID:   sk.SkillID,
			Name:      sk.Name,
			Source:    sk.Source,
			Installs:  sk.Installs,
			Installed: installed[sk.Name],
		}
		results = append(results, entry)
	}

	writeJSON(w, map[string]any{
		"skills": results,
		"count":  catalog.Count,
	})
}

func (s *Server) handleSkillsInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.ID == "" {
		writeErr(w, 400, "id required")
		return
	}

	cmd := exec.Command("npx", "-y", "skills", "add", req.ID, "-g", "-y")
	out, err := cmd.CombinedOutput()
	if err != nil {
		writeErr(w, 500, "install failed: "+err.Error()+"\n"+string(out))
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}
