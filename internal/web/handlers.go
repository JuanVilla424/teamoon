package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/chat"
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
		ID int `json:"id"`
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
		s.setGenerating(found.ID)
		s.refreshAndBroadcast()
		go s.generatePlanAsync(found)
		writeJSON(w, map[string]string{"status": "generating"})

	case queue.StatePlanned:
		p, err := plan.ParsePlan(plan.PlanPath(found.ID))
		if err != nil {
			writeErr(w, 500, "plan parse error: "+err.Error())
			return
		}
		queue.UpdateState(found.ID, queue.StateRunning)
		s.store.engineMgr.Start(found, p, s.webSend(found.ID))
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
		s.store.engineMgr.Start(found, p, s.webSend(found.ID))
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
		s.refreshAndBroadcast()
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
5. Trim .github/workflows/: keep only ci.yml, %s, version-controller.yml, release-controller.yml — delete all others
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

func (s *Server) generatePlanAsync(t queue.Task) {
	defer s.clearGenerating(t.ID)

	prompt := fmt.Sprintf(
		"You may read files to understand the codebase, then create a plan.\n"+
			"Project path: /home/cloud-agent/Projects/%s\n\n"+
			"TASK: %s\n\n"+
			"INSTRUCTIONS:\n"+
			"1. Read the relevant source files to understand the current code\n"+
			"2. Then output your final plan as a TEXT MESSAGE (not a tool call)\n\n"+
			"Your FINAL message MUST be the plan in this markdown format:\n\n"+
			"# Plan: %s\n\n"+
			"## Analysis\n[what you found in the codebase]\n\n"+
			"## Steps\n\n### Step 1: [title]\n[instructions with specific files and changes]\nVerify: [verification]\n\n"+
			"### Step 2: [title]\n[instructions]\nVerify: [verification]\n\n"+
			"(2-5 steps total)\n\n"+
			"## Constraints\n- [constraints]\n\n"+
			"CRITICAL: Your very last message MUST be the markdown plan text, NOT a tool call.",
		t.Project, t.Description, t.Description,
	)

	s.store.logBuf.Add(logs.LogEntry{
		Time: time.Now(), TaskID: t.ID, Project: t.Project,
		Message: "Generating plan...", Level: logs.LevelInfo,
	})
	s.refreshAndBroadcast()

	env := filterEnvWeb(os.Environ(), "CLAUDECODE")
	cmd := exec.Command("claude",
		"-p", prompt,
		"--output-format", "json",
		"--max-turns", "50",
		"--dangerously-skip-permissions",
		"--no-session-persistence",
	)
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		s.store.logBuf.Add(logs.LogEntry{
			Time: time.Now(), TaskID: t.ID, Project: t.Project,
			Message: "Plan generation failed: " + err.Error(), Level: logs.LevelError,
		})
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
	s.refreshAndBroadcast()
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
	cmd := exec.Command("claude",
		"-p", promptBuf.String(),
		"--output-format", "stream-json",
		"--verbose",
		"--max-turns", "10",
		"--dangerously-skip-permissions",
		"--no-session-persistence",
	)
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
