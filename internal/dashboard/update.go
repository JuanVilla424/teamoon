package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/metrics"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/projects"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

type taskDoneMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showDetail {
			return m.handleDetailKey(msg)
		}
		if m.showPlan {
			return m.handlePlanKey(msg)
		}
		if m.showMenu {
			return m.handleMenuKey(msg)
		}
		return m.handleMainKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if m.showPlan && m.planContent != "" {
			m.planLines = renderMarkdownLines(m.planContent, m.width-8)
		}

	case refreshMsg:
		return m, fetchData(m.cfg)

	case tickMsg:
		return m, tea.Batch(
			fetchData(m.cfg),
			tickEvery(time.Duration(m.cfg.RefreshIntervalSec)*time.Second),
		)

	case engine.LogMsg:
		m.logBuf.Add(msg.Entry)
		m.logEntries = m.logBuf.Snapshot()
		return m, listenChannel(m.msgChan)

	case engine.TaskStateMsg:
		for i := range m.tasks {
			if m.tasks[i].ID == msg.TaskID {
				m.tasks[i].State = msg.State
				break
			}
		}
		return m, tea.Batch(listenChannel(m.msgChan), fetchData(m.cfg))

	case engine.PlanGeneratedMsg:
		m.generatingPlan = false
		if msg.Err != nil {
			m.logBuf.Add(newLogEntry(msg.TaskID, "", "Plan generation failed: "+msg.Err.Error(), 3))
			m.logEntries = m.logBuf.Snapshot()
		} else {
			plan.SavePlan(msg.TaskID, msg.Content)
			queue.SetPlanFile(msg.TaskID, plan.PlanPath(msg.TaskID))
			m.logBuf.Add(newLogEntry(msg.TaskID, "", "Plan generated", 1))
			m.logEntries = m.logBuf.Snapshot()
		}
		return m, fetchData(m.cfg)

	case taskDoneMsg:
		return m, fetchData(m.cfg)

	case taskAddMsg:
		if msg.err != nil {
			m.menuStatus = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.menuStatus = "Task added"
		}
		return m, fetchData(m.cfg)

	case prInfoMsg:
		m.menuPRs = msg.prs
		m.menuDepBot = msg.depBot
		if msg.err != nil {
			m.menuStatus = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.menuStatus = ""
		}

	case mergeMsg:
		if msg.err != nil {
			m.menuStatus = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.menuStatus = fmt.Sprintf("Merged %d, failed %d", msg.merged, msg.failed)
		}
		return m, m.fetchPRInfo()

	case pullMsg:
		if msg.err != nil {
			m.menuStatus = fmt.Sprintf("Pull error: %v", msg.err)
		} else {
			m.menuStatus = fmt.Sprintf("Pull: %s", msg.output)
		}
		return m, fetchData(m.cfg)

	case dataMsg:
		m.today = msg.today
		m.week = msg.week
		m.month = msg.month
		m.session = msg.session
		m.projects = msg.projects
		m.tasks = msg.tasks
		m.err = msg.err
		m.cost = metrics.CalculateCost(m.today, m.week, m.month, m.cfg)
		if m.cursor >= len(m.tasks) && len(m.tasks) > 0 {
			m.cursor = len(m.tasks) - 1
		}
		if len(m.tasks) == 0 {
			m.cursor = 0
		}
		if m.projCursor >= len(m.projects) && len(m.projects) > 0 {
			m.projCursor = len(m.projects) - 1
		}
	}

	return m, nil
}

func (m Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.engineMgr.StopAll()
		return m, tea.Quit
	case tea.KeyCtrlA:
		return m, m.startAllAutopilot()
	case tea.KeyCtrlE:
		// kept for backward compat, real handler is "e" in KeyRunes
	case tea.KeyEsc:
		m.engineMgr.StopAll()
		return m, tea.Quit
	case tea.KeyTab:
		if m.focus == "queue" {
			m.focus = "projects"
		} else {
			m.focus = "queue"
		}
	case tea.KeyDown:
		if m.focus == "queue" {
			if len(m.tasks) > 0 && m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		} else {
			if len(m.projects) > 0 && m.projCursor < len(m.projects)-1 {
				m.projCursor++
			}
		}
	case tea.KeyUp:
		if m.focus == "queue" {
			if m.cursor > 0 {
				m.cursor--
			}
		} else {
			if m.projCursor > 0 {
				m.projCursor--
			}
		}
	case tea.KeyEnter:
		if m.focus == "queue" && len(m.tasks) > 0 && m.cursor < len(m.tasks) {
			return m.handleViewDetail()
		}
		if m.focus == "projects" && len(m.projects) > 0 && m.projCursor < len(m.projects) {
			m.showMenu = true
			m.menuPRs = nil
			m.menuDepBot = nil
			m.menuStatus = "Loading PRs..."
			return m, m.fetchPRInfo()
		}
	case tea.KeyRunes:
		key := strings.ToLower(string(msg.Runes))
		switch {
		case key == "q":
			m.engineMgr.StopAll()
			return m, tea.Quit
		case key == "r":
			return m, fetchData(m.cfg)
		case key == "a":
			return m.handleAutopilotKey()
		case key == "p":
			return m.handleViewPlan()
		case key == "d":
			if m.focus == "queue" && len(m.tasks) > 0 && m.cursor < len(m.tasks) {
				t := m.tasks[m.cursor]
				if m.engineMgr.IsRunning(t.ID) {
					m.engineMgr.Stop(t.ID)
				}
				id := t.ID
				return m, func() tea.Msg {
					err := queue.MarkDone(id)
					return taskDoneMsg{err: err}
				}
			}
		case key == "x":
			if m.focus == "queue" && len(m.tasks) > 0 && m.cursor < len(m.tasks) {
				t := m.tasks[m.cursor]
				if m.engineMgr.IsRunning(t.ID) {
					m.engineMgr.Stop(t.ID)
				}
				if t.PlanFile != "" {
					os.Remove(t.PlanFile)
				}
				id := t.ID
				return m, func() tea.Msg {
					queue.ResetPlan(id)
					return refreshMsg{}
				}
			}
		case key == "e":
			if m.focus == "queue" && len(m.tasks) > 0 && m.cursor < len(m.tasks) {
				id := m.tasks[m.cursor].ID
				return m, func() tea.Msg {
					queue.Archive(id)
					return taskDoneMsg{}
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleAutopilotKey() (tea.Model, tea.Cmd) {
	if m.focus != "queue" || len(m.tasks) == 0 || m.cursor >= len(m.tasks) {
		return m, nil
	}

	t := m.tasks[m.cursor]
	state := queue.EffectiveState(t)

	switch state {
	case queue.StatePending:
		if m.generatingPlan {
			return m, nil
		}
		m.generatingPlan = true
		m.generatingTaskID = t.ID
		return m, m.generatePlan(t)

	case queue.StatePlanned:
		return m, m.startAutopilot(t)

	case queue.StateRunning:
		m.engineMgr.Stop(t.ID)
		queue.UpdateState(t.ID, queue.StatePlanned)
		return m, fetchData(m.cfg)

	case queue.StateBlocked:
		queue.UpdateState(t.ID, queue.StatePlanned)
		return m, m.startAutopilot(t)
	}

	return m, nil
}

func (m Model) handleViewPlan() (tea.Model, tea.Cmd) {
	if m.focus != "queue" || len(m.tasks) == 0 || m.cursor >= len(m.tasks) {
		return m, nil
	}

	t := m.tasks[m.cursor]
	if plan.PlanExists(t.ID) {
		content, err := os.ReadFile(plan.PlanPath(t.ID))
		if err == nil {
			raw := string(content)
			m.showPlan = true
			m.planContent = raw
			m.planLines = renderMarkdownLines(raw, m.width-8)
			m.planTaskID = t.ID
			m.planScroll = 0
		}
	}
	return m, nil
}

func (m Model) handlePlanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc", "q":
		m.showPlan = false
		m.planScroll = 0
		return m, nil
	case "down", "j":
		m.planScroll++
	case "up", "k":
		m.planScroll--
	case "pgdown", " ":
		m.planScroll += 10
	case "pgup":
		m.planScroll -= 10
	case "home", "g":
		m.planScroll = 0
	case "end", "G":
		m.planScroll = 99999
	}

	// Clamp scroll to valid range
	maxShow := m.height - 6
	if maxShow < 5 {
		maxShow = 5
	}
	totalLines := len(m.planLines)
	maxScroll := totalLines - maxShow
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.planScroll > maxScroll {
		m.planScroll = maxScroll
	}
	if m.planScroll < 0 {
		m.planScroll = 0
	}

	return m, nil
}

func (m Model) handleViewDetail() (tea.Model, tea.Cmd) {
	if m.focus != "queue" || len(m.tasks) == 0 || m.cursor >= len(m.tasks) {
		return m, nil
	}

	t := m.tasks[m.cursor]
	entries := logs.ReadTaskLog(t.ID)

	var lines []string
	lines = append(lines, fmt.Sprintf("  Task #%d: %s", t.ID, t.Description))
	lines = append(lines, fmt.Sprintf("  Project: %s    Priority: %s    State: %s", t.Project, t.Priority, string(queue.EffectiveState(t))))
	lines = append(lines, fmt.Sprintf("  Created: %s", t.CreatedAt.Format("2006-01-02 15:04:05")))
	lines = append(lines, "")
	if t.PlanFile != "" {
		lines = append(lines, fmt.Sprintf("  Plan: %s", t.PlanFile))
	}
	if t.BlockReason != "" {
		lines = append(lines, fmt.Sprintf("  Block: %s", t.BlockReason))
	}
	lines = append(lines, "")
	lines = append(lines, "  ── Autopilot Log ──")
	lines = append(lines, "")

	if len(entries) == 0 {
		lines = append(lines, "  No log entries for this task")
	} else {
		for _, e := range entries {
			ts := e.Time.Format("15:04:05")
			msg := e.Message
			// Wrap long messages
			maxW := m.width - 20
			if maxW < 40 {
				maxW = 40
			}
			if len(msg) > maxW {
				// Multi-line
				for len(msg) > 0 {
					chunk := msg
					if len(chunk) > maxW {
						chunk = msg[:maxW]
					}
					msg = msg[len(chunk):]
					lines = append(lines, fmt.Sprintf("  [%s] %s", ts, chunk))
					ts = "        " // indent continuation
				}
			} else {
				lines = append(lines, fmt.Sprintf("  [%s] %s", ts, msg))
			}
		}
	}

	m.showDetail = true
	m.detailLines = lines
	m.detailTaskID = t.ID
	m.detailScroll = 0
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc", "q":
		m.showDetail = false
		m.detailScroll = 0
		return m, nil
	case "down", "j":
		m.detailScroll++
	case "up", "k":
		m.detailScroll--
	case "pgdown", " ":
		m.detailScroll += 10
	case "pgup":
		m.detailScroll -= 10
	case "home", "g":
		m.detailScroll = 0
	case "end", "G":
		m.detailScroll = 99999
	}

	maxShow := m.height - 6
	if maxShow < 5 {
		maxShow = 5
	}
	totalLines := len(m.detailLines)
	maxScroll := totalLines - maxShow
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}

	return m, nil
}

func (m Model) generatePlan(t queue.Task) tea.Cmd {
	taskID := t.ID
	desc := t.Description
	proj := t.Project

	return func() tea.Msg {
		prompt := fmt.Sprintf(
			"You are orchestrating a BMAD Party Mode planning session with 3 expert agents.\n"+
				"Each agent provides their perspective, then you synthesize into a final plan.\n"+
				"Do NOT use any tools. Output ONLY the final markdown plan.\n\n"+
				"TASK: %s\n"+
				"PROJECT: %s\n\n"+
				"--- COGNITIVE FRAMEWORK ---\n"+
				"The execution engine uses a 3-layer cognitive model:\n"+
				"- Layer 1 (Reflexive): Execute step, check exit code. Success = next step.\n"+
				"- Layer 2 (Deliberative): On failure, analyze error context and retry with fix.\n"+
				"- Layer 3 (Meta-cognitive): After 3 retries, block task for human review.\n"+
				"Each step runs as: claude -p \"prompt\" in the project directory.\n"+
				"Steps must be self-contained and independently verifiable.\n\n"+
				"--- AGENT DISCUSSION ---\n\n"+
				"**Winston (Architect)**: Analyze architecture, components, risk areas, and recovery strategies.\n"+
				"**John (PM)**: Define scope, acceptance criteria, and delivery priorities.\n"+
				"**Amelia (Dev)**: Break down into concrete coding steps with clear verify criteria.\n\n"+
				"--- SYNTHESIZED PLAN ---\n\n"+
				"After considering all 3 perspectives, output using EXACTLY this format:\n\n"+
				"# Plan: %s\n\n"+
				"## Agent Insights\n"+
				"- **Winston**: [architecture insight]\n"+
				"- **John**: [scope/priority insight]\n"+
				"- **Amelia**: [implementation insight]\n\n"+
				"## Steps\n\n"+
				"### Step 1: [title]\n[detailed instructions for Claude Code CLI]\nVerify: [concrete verification — test command, file check, or output match]\n\n"+
				"### Step 2: [title]\n[detailed instructions]\nVerify: [concrete verification]\n\n"+
				"(2-5 steps total, each independently executable)\n\n"+
				"## Dependencies\n"+
				"- %s/{other-project-name} (only if steps need to access files outside the main project directory)\n"+
				"(Omit this section entirely if no external directories are needed)\n\n"+
				"## Constraints\n- [constraints]\n\n"+
				"Output ONLY the markdown plan.",
			desc, proj, desc, m.cfg.ProjectsDir,
		)

		env := filterEnv(os.Environ(), "CLAUDECODE")
		cmd := exec.Command("claude",
			"-p", prompt,
			"--output-format", "json",
			"--max-turns", "1",
			"--no-session-persistence",
		)
		cmd.Env = env

		out, err := cmd.Output()
		if err != nil {
			return engine.PlanGeneratedMsg{TaskID: taskID, Err: err}
		}

		var result struct {
			Result  string `json:"result"`
			IsError bool   `json:"is_error"`
		}
		if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
			return engine.PlanGeneratedMsg{TaskID: taskID, Err: fmt.Errorf("parse error: %v", jsonErr)}
		}

		if result.IsError || result.Result == "" {
			return engine.PlanGeneratedMsg{TaskID: taskID, Err: fmt.Errorf("no plan generated")}
		}

		return engine.PlanGeneratedMsg{TaskID: taskID, Content: result.Result}
	}
}

func (m Model) startAutopilot(t queue.Task) tea.Cmd {
	task := t
	ch := m.msgChan
	mgr := m.engineMgr
	cfg := m.cfg

	return func() tea.Msg {
		p, err := plan.ParsePlan(plan.PlanPath(task.ID))
		if err != nil {
			return engine.TaskStateMsg{
				TaskID:  task.ID,
				State:   queue.StateBlocked,
				Message: "plan parse error: " + err.Error(),
			}
		}
		queue.UpdateState(task.ID, queue.StateRunning)
		mgr.Start(task, p, cfg, func(msg tea.Msg) { ch <- msg })
		return refreshMsg{}
	}
}

func (m Model) startAllAutopilot() tea.Cmd {
	var planned []queue.Task
	for _, t := range m.tasks {
		if queue.EffectiveState(t) == queue.StatePlanned && !m.engineMgr.IsRunning(t.ID) {
			planned = append(planned, t)
		}
	}
	if len(planned) == 0 {
		return nil
	}

	ch := m.msgChan
	mgr := m.engineMgr
	cfg := m.cfg

	return func() tea.Msg {
		for _, task := range planned {
			p, err := plan.ParsePlan(plan.PlanPath(task.ID))
			if err != nil {
				continue
			}
			queue.UpdateState(task.ID, queue.StateRunning)
			mgr.Start(task, p, cfg, func(msg tea.Msg) { ch <- msg })
		}
		return refreshMsg{}
	}
}

func (m Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inputMode {
		return m.handleInputKey(msg)
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.showMenu = false
		m.menuStatus = ""
		return m, nil
	case tea.KeyRunes:
		key := strings.ToLower(string(msg.Runes))
		switch key {
		case "q":
			m.showMenu = false
			m.menuStatus = ""
			return m, nil
		case "1":
			if len(m.menuDepBot) > 0 {
				p := m.projects[m.projCursor]
				depBot := m.menuDepBot
				m.menuStatus = "Merging..."
				return m, func() tea.Msg {
					merged, failed := 0, 0
					for _, pr := range depBot {
						if err := projects.MergePR(p.GitHubRepo, pr.Number); err != nil {
							failed++
						} else {
							merged++
						}
					}
					return mergeMsg{merged: merged, failed: failed}
				}
			}
		case "2":
			p := m.projects[m.projCursor]
			m.menuStatus = "Pulling..."
			return m, func() tea.Msg {
				out, err := projects.GitPull(p.Path)
				return pullMsg{output: out, err: err}
			}
		case "4":
			m.inputMode = true
			m.inputBuffer = ""
			m.inputPriority = "med"
			m.menuStatus = ""
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.inputMode = false
		m.inputBuffer = ""
		return m, nil
	case tea.KeyEnter:
		if m.inputBuffer != "" {
			p := m.projects[m.projCursor]
			desc := m.inputBuffer
			pri := m.inputPriority
			m.inputMode = false
			m.inputBuffer = ""
			m.menuStatus = "Adding task..."
			return m, func() tea.Msg {
				_, err := queue.Add(p.Name, desc, pri)
				return taskAddMsg{err: err}
			}
		}
	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			runes := []rune(m.inputBuffer)
			m.inputBuffer = string(runes[:len(runes)-1])
		}
	case tea.KeyUp:
		switch m.inputPriority {
		case "low":
			m.inputPriority = "med"
		case "med":
			m.inputPriority = "high"
		case "high":
			m.inputPriority = "low"
		}
	case tea.KeyDown:
		switch m.inputPriority {
		case "high":
			m.inputPriority = "med"
		case "med":
			m.inputPriority = "low"
		case "low":
			m.inputPriority = "high"
		}
	case tea.KeySpace:
		m.inputBuffer += " "
	case tea.KeyRunes:
		m.inputBuffer += string(msg.Runes)
	}
	return m, nil
}

func (m Model) fetchPRInfo() tea.Cmd {
	p := m.projects[m.projCursor]
	repo := p.GitHubRepo
	return func() tea.Msg {
		prs, err := projects.FetchPRs(repo)
		if err != nil {
			return prInfoMsg{err: err}
		}
		depBot := projects.FilterDependabot(prs)
		return prInfoMsg{prs: prs, depBot: depBot}
	}
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

func newLogEntry(taskID int, project, message string, level int) logs.LogEntry {
	return logs.LogEntry{
		Time:    time.Now(),
		TaskID:  taskID,
		Project: project,
		Message: message,
		Level:   logs.LogLevel(level),
	}
}
