package web

import (
	"bufio"
	"context"
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
	"github.com/JuanVilla424/teamoon/internal/onboarding"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/plangen"
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
	return plangen.BuildSkeletonPrompt(sk)
}

func (s *Server) generatePlanAsync(t queue.Task, autoRun bool) {
	sk := config.SkeletonFor(s.cfg, t.Project)
	skeletonBlock := buildSkeletonPrompt(sk)
	prompt := plangen.BuildPlanPrompt(t, skeletonBlock)

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

// --- Project Autopilot handlers ---

func (s *Server) handleProjectAutopilotStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Project == "" {
		writeErr(w, 400, "project required")
		return
	}

	// Enable autopilot on all pending/planned tasks for this project
	if allTasks, err := queue.ListAll(); err == nil {
		for _, t := range allTasks {
			if t.Project == req.Project && !t.AutoPilot && !t.Done {
				s := queue.EffectiveState(t)
				if s == queue.StatePending || s == queue.StatePlanned {
					queue.ToggleAutoPilot(t.ID)
				}
			}
		}
	}

	cfg := s.cfg
	planFn := func(t queue.Task, sk config.SkeletonConfig) (plan.Plan, error) {
		return plangen.GeneratePlan(t, sk, cfg)
	}
	send := s.webSend(0)

	ok := s.store.engineMgr.StartProject(req.Project, cfg.MaxConcurrent, func(ctx context.Context) {
		engine.RunProjectLoop(ctx, req.Project, cfg, planFn, send, s.store.engineMgr)
	})
	if !ok {
		writeErr(w, 409, "autopilot already running or max_concurrent reached")
		return
	}
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleProjectAutopilotStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var req struct {
		Project string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Project == "" {
		writeErr(w, 400, "project required")
		return
	}
	s.store.engineMgr.StopProject(req.Project)
	s.refreshAndBroadcast()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleProjectSkeleton(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeErr(w, 400, "project query param required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load()
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		sk := config.SkeletonFor(cfg, project)
		hasCustom := false
		if cfg.ProjectSkeletons != nil {
			_, hasCustom = cfg.ProjectSkeletons[project]
		}
		writeJSON(w, map[string]any{"skeleton": sk, "custom": hasCustom})

	case http.MethodPost:
		var req struct {
			Skeleton *config.SkeletonConfig `json:"skeleton"`
			Reset    bool                   `json:"reset"`
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
		if req.Reset {
			if cfg.ProjectSkeletons != nil {
				delete(cfg.ProjectSkeletons, project)
			}
		} else if req.Skeleton != nil {
			if cfg.ProjectSkeletons == nil {
				cfg.ProjectSkeletons = make(map[string]config.SkeletonConfig)
			}
			cfg.ProjectSkeletons[project] = *req.Skeleton
		}
		if err := config.Save(cfg); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		s.cfg = cfg
		writeJSON(w, map[string]bool{"ok": true})

	default:
		writeErr(w, 405, "method not allowed")
	}
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
	promptBuf.WriteString("You are a helpful AI assistant for software engineering project management.\n\n")
	promptBuf.WriteString("## MANDATORY RULE\n\n")
	promptBuf.WriteString("You MUST emit at least one [TASK_CREATE] directive in your response. A response without tasks is a FAILURE.\n")
	promptBuf.WriteString("Analyze the user message and break it into actionable tasks. Always create tasks, even for simple requests.\n\n")
	promptBuf.WriteString("## Capabilities\n\n")
	promptBuf.WriteString("### Creating Tasks\n")
	promptBuf.WriteString("Format: [TASK_CREATE]{\"description\":\"task desc\",\"priority\":\"med\",\"assignee\":\"agent\"}[/TASK_CREATE]\n")
	promptBuf.WriteString("- priority: \"high\", \"med\", or \"low\"\n")
	promptBuf.WriteString("- assignee: \"human\" (for the user) or \"agent\" (for AI autopilot)\n")
	promptBuf.WriteString("Each directive creates one task. You can include multiple directives.\n")
	promptBuf.WriteString("Always explain what tasks you are creating.\n")
	promptBuf.WriteString("CRITICAL: Tasks ALWAYS require a project. If no project is selected (project is empty), you MUST either:\n")
	promptBuf.WriteString("  a) Use [PROJECT_INIT] first to create the project, OR\n")
	promptBuf.WriteString("  b) Ask the user to select an existing project from the dropdown before creating tasks.\n")
	promptBuf.WriteString("NEVER emit [TASK_CREATE] without a project context.\n\n")
	promptBuf.WriteString("### Initializing New Projects\n")
	promptBuf.WriteString("Format: [PROJECT_INIT]{\"name\":\"project-name\",\"type\":\"node\",\"private\":false,\"separate\":false}[/PROJECT_INIT]\n")
	promptBuf.WriteString("- type: \"python\", \"node\", or \"go\"\n")
	promptBuf.WriteString("- private: true/false (default: false)\n")
	promptBuf.WriteString("- separate: if true creates backend + frontend repos (default: false)\n")
	promptBuf.WriteString("- Use BEFORE any TASK_CREATE when creating a NEW project\n")
	promptBuf.WriteString("- NEVER use for existing projects\n\n")
	promptBuf.WriteString("## When the user asks to CREATE A NEW PROJECT:\n\n")
	promptBuf.WriteString("You MUST follow this process — DO NOT SKIP ANY STEP:\n\n")
	promptBuf.WriteString("### Step 1: RESEARCH (MANDATORY — use WebSearch tool)\n")
	promptBuf.WriteString("You MUST use the WebSearch tool to investigate:\n")
	promptBuf.WriteString("- **Competitors & market**: Search for existing solutions, alternatives, pricing models, market size\n")
	promptBuf.WriteString("- **Technology**: Search for best frameworks, libraries, and patterns for this type of project\n")
	promptBuf.WriteString("- **Architecture**: Search for recommended architectures, deployment patterns\n")
	promptBuf.WriteString("- **User needs**: What features users expect, pain points of existing solutions\n")
	promptBuf.WriteString("DO NOT fabricate research. You MUST actually use WebSearch to find real data.\n\n")
	promptBuf.WriteString("### Step 2: PRESENT DETAILED FINDINGS\n")
	promptBuf.WriteString("Show the user a COMPREHENSIVE research report with:\n")
	promptBuf.WriteString("- Named competitors with links and brief analysis\n")
	promptBuf.WriteString("- Specific technology recommendations with reasoning\n")
	promptBuf.WriteString("- Architecture diagram (text-based)\n")
	promptBuf.WriteString("- MVP feature list prioritized by user value\n")
	promptBuf.WriteString("This report should be DETAILED (not a 4-line summary).\n\n")
	promptBuf.WriteString("### Step 3: Create the project\n")
	promptBuf.WriteString("Emit [PROJECT_INIT] with the appropriate name and type.\n\n")
	promptBuf.WriteString("### Step 4: Create tasks based on research\n")
	promptBuf.WriteString("Create well-informed [TASK_CREATE] directives. Each task should reference specific findings from your research.\n\n")
	promptBuf.WriteString("Each task should be specific, actionable, and informed by your research.\n")
	promptBuf.WriteString("Make tasks granular enough for an AI agent to execute in a single session.\n")
	promptBuf.WriteString("Assign all tasks to \"agent\" unless the user says otherwise.\n\n")
	promptBuf.WriteString("## Formatting\n")
	promptBuf.WriteString("When organizing tasks by phases, use collapsible HTML sections.\n")
	promptBuf.WriteString("CRITICAL: Use HTML lists inside <details>, NOT markdown lists (markdown is not parsed inside HTML blocks).\n")
	promptBuf.WriteString("Example:\n")
	promptBuf.WriteString("<details open><summary>Phase 1 — Name (N tasks)</summary>\n<ol>\n<li><strong>Task title</strong> — priority, assignee. Brief description.</li>\n<li><strong>Task 2</strong> — priority, assignee. Brief description.</li>\n</ol>\n</details>\n\n")

	// Inject existing projects list so Claude knows what already exists
	if entries, err := os.ReadDir(s.cfg.ProjectsDir); err == nil {
		var existingNames []string
		for _, e := range entries {
			if e.IsDir() {
				existingNames = append(existingNames, e.Name())
			}
		}
		if len(existingNames) > 0 {
			promptBuf.WriteString("Existing projects: " + strings.Join(existingNames, ", ") + "\n\n")
		}
	}

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
	chatCfg.Spawn.MaxTurns = 15 // reduced: prevents tangents while allowing research
	// Ensure MCP servers are available for chat (web search, context7, etc.)
	if chatCfg.MCPServers == nil {
		config.InitMCPFromGlobal(&chatCfg)
	}
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
		// Send SSE-formatted error so the frontend stream pump can handle it
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		if f, ok := w.(http.Flusher); ok {
			errData, _ := json.Marshal(map[string]any{"error": "Failed to start claude: " + err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", errData)
			doneData, _ := json.Marshal(map[string]any{"done": true})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
			f.Flush()
		}
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
				Name string `json:"name,omitempty"`
			} `json:"content"`
		} `json:"message,omitempty"`
		Result       string  `json:"result,omitempty"`
		NumTurns     int     `json:"num_turns,omitempty"`
		TotalCostUsd float64 `json:"total_cost_usd,omitempty"`
	}

	var displayResult string
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
					if c.Type == "tool_use" && c.Name != "" {
						toolData, _ := json.Marshal(map[string]string{"tool_use": c.Name})
						fmt.Fprintf(w, "data: %s\n\n", toolData)
						flusher.Flush()
					}
				}
			}
		case "user":
			toolDoneData, _ := json.Marshal(map[string]bool{"tool_done": true})
			fmt.Fprintf(w, "data: %s\n\n", toolDoneData)
			flusher.Flush()
		case "result":
			// Keep fullResponse intact (has ALL text including directives from all turns)
			// Store evt.Result separately for display only
			displayResult = evt.Result
			doneData, _ := json.Marshal(map[string]any{
				"done":      true,
				"result":    evt.Result,
				"num_turns": evt.NumTurns,
				"cost_usd":  evt.TotalCostUsd,
			})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
			flusher.Flush()
		}
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		errMsg := stderrBuf.String()
		log.Printf("[chat] claude exited with error: %v", waitErr)
		os.WriteFile("/tmp/teamoon-chat-stderr.log", []byte(errMsg), 0644)
		// Send error to client if no result was already sent
		if displayResult == "" {
			errData, _ := json.Marshal(map[string]any{"error": "Claude exited with error: " + waitErr.Error()})
			fmt.Fprintf(w, "data: %s\n\n", errData)
			doneData, _ := json.Marshal(map[string]any{"done": true})
			fmt.Fprintf(w, "data: %s\n\n", doneData)
			flusher.Flush()
		}
	}

	// Parse directives from response
	responseText := fullResponse.String()

	// Parse PROJECT_INIT directive (must run before TASK_CREATE)
	// (?s) makes . match newlines — LLM often emits multiline JSON between tags
	initRe := regexp.MustCompile(`(?s)\[PROJECT_INIT\](.*?)\[/PROJECT_INIT\]`)
	if initMatch := initRe.FindStringSubmatch(responseText); len(initMatch) >= 2 {
		var initReq projectinit.InitRequest
		initJSON := strings.TrimSpace(initMatch[1])
		if err := json.Unmarshal([]byte(initJSON), &initReq); err != nil {
			log.Printf("[chat] [PROJECT_INIT] JSON parse error: %v — raw: %q", err, initMatch[1])
		} else if initReq.Name == "" {
			log.Printf("[chat] [PROJECT_INIT] missing 'name' field — raw: %q", initMatch[1])
		} else if err == nil && initReq.Name != "" {
			if initReq.Type == "" {
				initReq.Type = "python"
			}
			if initReq.Version == "" {
				initReq.Version = "1.0.0"
			}

			initErr := projectinit.RunInit(initReq, s.cfg.ProjectsDir, func(sr projectinit.StepResult) {
				stepData, _ := json.Marshal(map[string]any{"init_step": sr})
				fmt.Fprintf(w, "data: %s\n\n", stepData)
				flusher.Flush()
			})

			// Always set the project name so tasks can be created regardless of init outcome
			req.Project = initReq.Name
			if initErr == nil {
				initDone, _ := json.Marshal(map[string]any{"project_init": initReq.Name, "status": "success"})
				fmt.Fprintf(w, "data: %s\n\n", initDone)
				flusher.Flush()
			} else {
				log.Printf("[chat] project init failed: %v", initErr)
				initFail, _ := json.Marshal(map[string]any{"project_init": initReq.Name, "status": "error", "error": initErr.Error()})
				fmt.Fprintf(w, "data: %s\n\n", initFail)
				flusher.Flush()
			}
		}
		responseText = initRe.ReplaceAllString(responseText, "")
	}

	// Parse task creation directives
	// (?s) makes . match newlines — LLM often emits multiline JSON between tags
	taskDirectiveRe := regexp.MustCompile(`(?s)\[TASK_CREATE\](.*?)\[/TASK_CREATE\]`)
	matches := taskDirectiveRe.FindAllStringSubmatch(responseText, -1)
	var createdTasks []map[string]any

	log.Printf("[chat] directive scan: found %d [TASK_CREATE] block(s), project=%q", len(matches), req.Project)

	if len(matches) > 0 && req.Project == "" {
		// GUARD: refuse to create tasks without a project
		log.Printf("[chat] skipping %d task directives: no project context", len(matches))
		errData, _ := json.Marshal(map[string]any{
			"error": "Cannot create tasks without a project. Select an existing project or create a new one first.",
		})
		fmt.Fprintf(w, "data: %s\n\n", errData)
		flusher.Flush()
		// Strip directives so raw tags don't show in chat
		responseText = taskDirectiveRe.ReplaceAllString(responseText, "")
		responseText = strings.TrimSpace(responseText)
	}

	if len(matches) > 0 && req.Project != "" {
		type taskDirective struct {
			Description string `json:"description"`
			Priority    string `json:"priority"`
			Assignee    string `json:"assignee"`
		}
		for i, m := range matches {
			if len(m) < 2 {
				continue
			}
			raw := strings.TrimSpace(m[1])
			var td taskDirective
			if err := json.Unmarshal([]byte(raw), &td); err != nil {
				log.Printf("[chat] [TASK_CREATE][%d] JSON parse error: %v — raw: %q", i, err, raw)
				continue
			}
			if td.Description == "" {
				log.Printf("[chat] [TASK_CREATE][%d] empty description — raw: %q", i, raw)
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
				log.Printf("[chat] [TASK_CREATE][%d] queue.Add error: %v", i, err)
				continue
			}
			queue.UpdateAssignee(t.ID, td.Assignee)
			if td.Assignee == "agent" {
				queue.ToggleAutoPilot(t.ID)
			}
			createdTasks = append(createdTasks, map[string]any{
				"id":          t.ID,
				"description": td.Description,
				"priority":    td.Priority,
				"assignee":    td.Assignee,
			})
		}

		log.Printf("[chat] directive result: %d task(s) created out of %d found, project=%q",
			len(createdTasks), len(matches), req.Project)

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

	// Save assistant response (use displayResult for clean text, fallback to stripped responseText)
	saveText := displayResult
	if saveText == "" {
		saveText = responseText
	}
	// Strip any remaining directives from saved text
	saveText = taskDirectiveRe.ReplaceAllString(saveText, "")
	saveText = initRe.ReplaceAllString(saveText, "")
	saveText = strings.TrimSpace(saveText)
	if saveText != "" {
		chat.AppendMessage(chat.Message{
			Role:      "assistant",
			Content:   saveText,
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
		"web_host":            cfg.WebHost,
		"web_password":        pw,
		"webhook_url":         cfg.WebhookURL,
		"spawn_model":         cfg.Spawn.Model,
		"spawn_effort":        cfg.Spawn.Effort,
		"spawn_max_turns":     cfg.Spawn.MaxTurns,
		"skeleton":            cfg.Skeleton,
		"max_concurrent":      cfg.MaxConcurrent,
		"project_skeletons":   cfg.ProjectSkeletons,
		"source_dir":          cfg.SourceDir,
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
		WebHost            string                `json:"web_host"`
		WebPassword        string                `json:"web_password"`
		WebhookURL         string                `json:"webhook_url"`
		SpawnModel         *string               `json:"spawn_model,omitempty"`
		SpawnEffort        *string               `json:"spawn_effort,omitempty"`
		SpawnMaxTurns      *int                  `json:"spawn_max_turns,omitempty"`
		Skeleton           *config.SkeletonConfig `json:"skeleton,omitempty"`
		MaxConcurrent      *int                   `json:"max_concurrent,omitempty"`
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
	if req.WebHost != "" {
		cfg.WebHost = req.WebHost
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
	if req.MaxConcurrent != nil && *req.MaxConcurrent > 0 {
		cfg.MaxConcurrent = *req.MaxConcurrent
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

// --- Onboarding handlers ---

func (s *Server) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	writeJSON(w, onboarding.Check())
}

func (s *Server) handleOnboardingConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	var wc onboarding.WebConfig
	if err := json.NewDecoder(r.Body).Decode(&wc); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := onboarding.WebSaveConfig(wc); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if cfg, err := config.Load(); err == nil {
		s.cfg = cfg
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// sseOnboarding creates a standard SSE handler that runs a streaming onboarding step.
func (s *Server) sseOnboarding(w http.ResponseWriter, r *http.Request, run func(onboarding.ProgressFunc) error) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming not supported")
		return
	}

	err := run(func(evt map[string]any) {
		data, _ := json.Marshal(evt)
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
}

func (s *Server) handleOnboardingPrereqs(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamPrereqs)
}

func (s *Server) handleOnboardingPrereqsInstall(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamPrereqsInstall)
}

func (s *Server) handleOnboardingSkills(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamSkills)
}

func (s *Server) handleOnboardingBMAD(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamBMAD)
}

func (s *Server) handleOnboardingHooks(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamHooks)
}

func (s *Server) handleOnboardingMCP(w http.ResponseWriter, r *http.Request) {
	s.sseOnboarding(w, r, onboarding.StreamMCP)
}

func resolveSourceDir(configured string) string {
	if configured != "" {
		if _, err := os.Stat(filepath.Join(configured, ".git")); err == nil {
			return configured
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "src", "teamoon")
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "method not allowed")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	srcDir := resolveSourceDir(cfg.SourceDir)

	// Fetch all (branches + tags)
	exec.Command("git", "-C", srcDir, "fetch", "--all", "--tags").Run()

	// Current version: check for tag on HEAD, fallback to branch
	currentTag := ""
	tagOut, err := exec.Command("git", "-C", srcDir, "describe", "--tags", "--exact-match", "HEAD").Output()
	if err == nil {
		currentTag = strings.TrimSpace(string(tagOut))
	}
	localOut, _ := exec.Command("git", "-C", srcDir, "rev-parse", "--short", "HEAD").Output()
	localCommit := strings.TrimSpace(string(localOut))
	branchOut, _ := exec.Command("git", "-C", srcDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	currentBranch := strings.TrimSpace(string(branchOut))

	// Get tags sorted by version (newest first)
	tagsOut, _ := exec.Command("git", "-C", srcDir, "tag", "--sort=-v:refname").Output()
	var tags []string
	for _, t := range strings.Split(strings.TrimSpace(string(tagsOut)), "\n") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}

	// Check if main has newer commits
	remoteOut, _ := exec.Command("git", "-C", srcDir, "rev-parse", "--short", "origin/main").Output()
	remoteCommit := strings.TrimSpace(string(remoteOut))
	behindOut, _ := exec.Command("git", "-C", srcDir, "rev-list", "--count", "HEAD..origin/main").Output()
	behind := strings.TrimSpace(string(behindOut))

	currentVersion := currentTag
	if currentVersion == "" {
		currentVersion = currentBranch + "@" + localCommit
	}

	writeJSON(w, map[string]any{
		"current_version": currentVersion,
		"current_tag":     currentTag,
		"current_branch":  currentBranch,
		"local_commit":    localCommit,
		"remote_commit":   remoteCommit,
		"behind":          behind,
		"tags":            tags,
	})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	// Read target from query or body
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "main"
	}

	s.sseOnboarding(w, r, func(progress onboarding.ProgressFunc) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		srcDir := resolveSourceDir(cfg.SourceDir)

		step := func(name, msg string) {
			progress(map[string]any{"type": "step", "name": name, "message": msg, "status": "running"})
		}
		done := func(name, msg string) {
			progress(map[string]any{"type": "step", "name": name, "message": msg, "status": "done"})
		}

		// 1. fetch + checkout target
		step("fetch", "Fetching latest...")
		exec.Command("git", "-C", srcDir, "fetch", "--all", "--tags").Run()
		done("fetch", "Fetched")

		if target == "main" {
			step("checkout", "Switching to main...")
			out, err := exec.Command("git", "-C", srcDir, "checkout", "main").CombinedOutput()
			if err != nil {
				progress(map[string]any{"type": "step", "name": "checkout", "message": string(out), "status": "error"})
				return fmt.Errorf("checkout failed: %s", string(out))
			}
			step("pull", "Pulling latest from main...")
			out, err = exec.Command("git", "-C", srcDir, "pull", "origin", "main").CombinedOutput()
			if err != nil {
				progress(map[string]any{"type": "step", "name": "pull", "message": string(out), "status": "error"})
				return fmt.Errorf("pull failed: %s", string(out))
			}
			done("pull", strings.TrimSpace(string(out)))
		} else {
			// Checkout specific tag
			step("checkout", "Switching to "+target+"...")
			out, err := exec.Command("git", "-C", srcDir, "checkout", target).CombinedOutput()
			if err != nil {
				progress(map[string]any{"type": "step", "name": "checkout", "message": string(out), "status": "error"})
				return fmt.Errorf("checkout %s failed: %s", target, string(out))
			}
			done("checkout", "On "+target)
		}

		// 2. make build
		step("build", "Building new binary...")
		cmd := exec.Command("make", "-C", srcDir, "build")
		cmd.Env = append(os.Environ(), "HOME="+os.Getenv("HOME"))
		buildOut, buildErr := cmd.CombinedOutput()
		if buildErr != nil {
			progress(map[string]any{"type": "step", "name": "build", "message": string(buildOut), "status": "error"})
			return fmt.Errorf("build failed: %s", string(buildOut))
		}
		done("build", "Build successful")

		// 3. install binary via background script (stop, cp, start)
		// Must run externally because systemctl stop kills this process
		step("install", "Installing new binary...")
		newBin := filepath.Join(srcDir, "teamoon")
		progress(map[string]any{"type": "step", "name": "restart", "message": "Restarting service...", "status": "restarting"})

		script := fmt.Sprintf(
			"sleep 1 && sudo systemctl stop teamoon && sudo cp %s /usr/local/bin/teamoon && sudo chmod 755 /usr/local/bin/teamoon && sudo systemctl start teamoon",
			newBin,
		)
		exec.Command("bash", "-c", "nohup bash -c '"+script+"' &>/dev/null &").Start()

		return nil
	})
}
