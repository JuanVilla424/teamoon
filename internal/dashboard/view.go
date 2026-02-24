package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

var (
	Version  string
	BuildNum string
)

func (m Model) View() string {
	if !m.ready {
		return "Loading teamoon..."
	}

	w := m.width
	if w < 60 {
		w = 60
	}
	h := m.height
	if h < 20 {
		h = 20
	}

	if m.showDetail {
		return m.renderDetailOverlay(w, h)
	}

	if m.showPlan {
		return m.renderPlanOverlay(w, h)
	}

	if m.showMenu {
		return m.renderMenuOverlay(w, h)
	}

	var b strings.Builder

	// Header (3 lines: title+status, model info, blank)
	header := titleStyle.Render(fmt.Sprintf(" teamoon v%s b%s ", Version, BuildNum))
	status := fmt.Sprintf("  %s Running    %s", runningDot, time.Now().Format("02 Jan 2006 15:04"))
	modelInfo := subtitleStyle.Render(fmt.Sprintf("  Plan: %s  Exec: %s  Effort: %s", m.planModel, m.execModel, m.effort))

	b.WriteString(header + status + "\n")
	b.WriteString(modelInfo + "\n\n")

	// TOKENS + COST side by side
	halfW := (w - 5) / 2
	if halfW < 28 {
		halfW = 28
	}
	barWidth := halfW - 20
	if barWidth < 8 {
		barWidth = 8
	}

	tokensContent := m.renderTokens(halfW, barWidth)
	costContent := m.renderCost(halfW, barWidth)

	tokensPanel := borderStyle.Width(halfW).Render(tokensContent)
	costPanel := borderStyle.Width(halfW).Render(costContent)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tokensPanel, " ", costPanel))
	b.WriteString("\n")

	// Calculate available height: header(3) + tokens(10) + gaps(4) + borders(6) + help(2) + logs panel overhead(1)
	fixedLines := 28
	logsLines := 6
	availableLines := h - fixedLines - logsLines
	if availableLines < 6 {
		availableLines = 6
	}

	taskCount := len(m.tasks)
	queueMax := taskCount
	if queueMax > availableLines-3 {
		queueMax = availableLines - 3
	}
	if queueMax < 3 {
		queueMax = 3
	}
	projMax := availableLines - queueMax
	if projMax < 3 {
		projMax = 3
	}

	// QUEUE
	queueContent := m.renderQueue(w-4, queueMax)
	queuePanel := borderStyle.Width(w - 4).Render(queueContent)
	b.WriteString(queuePanel)
	b.WriteString("\n")

	// AUTOPILOT LOGS
	logsContent := m.renderLogs(w-4, 5)
	logsPanel := logBorderStyle.Width(w - 4).Render(logsContent)
	b.WriteString(logsPanel)
	b.WriteString("\n")

	// PROJECTS
	projContent := m.renderProjects(w-4, projMax)
	projPanel := borderStyle.Width(w - 4).Render(projContent)
	b.WriteString(projPanel)
	b.WriteString("\n")

	// Help - 2 compact lines
	b.WriteString(helpStyle.Render(" esc: quit  tab: switch  ↑↓: nav  r: refresh"))
	b.WriteString("\n")
	if m.focus == "queue" {
		b.WriteString(helpStyle.Render(" enter: detail  a: run  p: plan  d: done  x: replan  e: archive  ctrl+a: all"))
	} else {
		b.WriteString(helpStyle.Render(" enter: actions"))
	}

	return b.String()
}

func (m Model) renderTokens(width int, barWidth int) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render("TOKENS") + "\n")
	b.WriteString(fmt.Sprintf("  Input:      %s\n", formatNum(m.month.Input)))
	b.WriteString(fmt.Sprintf("  Output:     %s\n", formatNum(m.month.Output)))
	b.WriteString(fmt.Sprintf("  Cache Read: %s\n", formatNum(m.month.CacheRead)))
	b.WriteString(fmt.Sprintf("  Total:      %s\n", formatNum(m.month.Total)))

	pct := m.session.ContextPercent
	ctx := formatNum(m.session.ContextTokens)
	lim := formatNum(m.session.ContextLimit)
	var pctLabel string
	switch {
	case pct >= 90:
		pctLabel = highStyle.Render(fmt.Sprintf("%.0f%% COMPACTING", pct))
	case pct >= 80:
		pctLabel = highStyle.Render(fmt.Sprintf("%.0f%%", pct))
	case pct >= 60:
		pctLabel = medStyle.Render(fmt.Sprintf("%.0f%%", pct))
	default:
		pctLabel = fmt.Sprintf("%.0f%%", pct)
	}
	b.WriteString(fmt.Sprintf("  %s %s context (%s / %s)", contextBar(pct, barWidth), pctLabel, ctx, lim))

	return b.String()
}

func (m Model) renderCost(width int, barWidth int) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render("USAGE") + "\n")

	b.WriteString(fmt.Sprintf("  Today:      %d sessions\n", m.cost.SessionsToday))
	b.WriteString(fmt.Sprintf("              %s output\n", formatNum(m.cost.OutputToday)))
	b.WriteString(fmt.Sprintf("  This Week:  %d sessions\n", m.cost.SessionsWeek))
	b.WriteString(fmt.Sprintf("              %s output\n", formatNum(m.cost.OutputWeek)))
	b.WriteString(fmt.Sprintf("  This Month: %d sessions\n", m.cost.SessionsMonth))
	b.WriteString(fmt.Sprintf("              %s output", formatNum(m.cost.OutputMonth)))

	return b.String()
}

func (m Model) renderQueue(width int, maxItems int) string {
	var b strings.Builder
	title := "QUEUE — Pending Tasks"
	if m.focus == "queue" {
		title = "» " + title
	}
	b.WriteString(panelTitleStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasks))) + "\n")

	if len(m.tasks) == 0 {
		b.WriteString("  No pending tasks\n")
		return b.String()
	}

	// Reserve lines for "more above/below" indicators
	visibleItems := maxItems
	if len(m.tasks) > maxItems {
		visibleItems = maxItems - 2
		if visibleItems < 1 {
			visibleItems = 1
		}
	}

	// Viewport scrolling
	startIdx, endIdx := viewport(m.cursor, len(m.tasks), visibleItems)

	// Column widths
	colState := 3
	colPri := 4
	colProj := 20
	colAgo := 8
	colDesc := width - colState - colPri - colProj - colAgo - 8
	if colDesc < 10 {
		colDesc = 10
	}

	for i := startIdx; i < endIdx; i++ {
		t := m.tasks[i]
		pri := formatPriority(t.Priority)
		ago := timeAgo(t.CreatedAt)
		stateTag := formatStateTag(t, m.engineMgr.IsRunning(t.ID), m.generatingPlan && m.generatingTaskID == t.ID)

		proj := t.Project
		if len(proj) > colProj {
			proj = proj[:colProj-1] + "."
		}

		desc := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(t.Description)
		if len(desc) > colDesc {
			desc = desc[:colDesc-3] + "..."
		}

		prefix := "  "
		if m.focus == "queue" && i == m.cursor {
			prefix = "> "
		}

		line := prefix +
			padRaw(stateTag, colState) + " " +
			padRaw(pri, colPri) + " " +
			padStr(proj, colProj) + " " +
			padStr(desc, colDesc) + " " +
			ago

		if m.focus == "queue" && i == m.cursor {
			line = cursorStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}

	if startIdx > 0 {
		b.WriteString(fmt.Sprintf("  ↑ %d more above\n", startIdx))
	}
	if endIdx < len(m.tasks) {
		b.WriteString(fmt.Sprintf("  ↓ %d more below\n", len(m.tasks)-endIdx))
	}

	return b.String()
}

func formatStateTag(t queue.Task, isRunning bool, isGenerating bool) string {
	if isGenerating {
		return plannedTagStyle.Render("GEN")
	}

	state := queue.EffectiveState(t)
	if isRunning {
		state = queue.StateRunning
	}

	switch state {
	case queue.StatePlanned:
		return plannedTagStyle.Render("PLN")
	case queue.StateRunning:
		return runningTagStyle.Render("RUN")
	case queue.StateDone:
		return inactiveStyle.Render("DON")
	default:
		return autoPilotOffStyle.Render("OFF")
	}
}

func (m Model) renderLogs(width int, maxLines int) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render("AUTOPILOT LOGS") + "\n")

	if len(m.logEntries) == 0 {
		b.WriteString(inactiveStyle.Render("  No autopilot activity") + "\n")
		for i := 0; i < maxLines-2; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	// Show last N entries
	start := 0
	if len(m.logEntries) > maxLines-1 {
		start = len(m.logEntries) - (maxLines - 1)
	}
	entries := m.logEntries[start:]

	for _, e := range entries {
		ts := e.Time.Format("15:04:05")
		proj := e.Project
		if len(proj) > 12 {
			proj = proj[:12]
		}

		msg := e.Message
		maxMsg := width - 30
		if maxMsg < 20 {
			maxMsg = 20
		}
		if len(msg) > maxMsg {
			msg = msg[:maxMsg-3] + "..."
		}

		line := fmt.Sprintf("  [%s] %-12s #%d: %s", ts, proj, e.TaskID, msg)

		switch e.Level {
		case logs.LevelSuccess:
			b.WriteString(activeStyle.Render(line) + "\n")
		case logs.LevelWarn:
			b.WriteString(medStyle.Render(line) + "\n")
		case logs.LevelError:
			b.WriteString(staleStyle.Render(line) + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}

	// Pad remaining lines
	shown := len(entries) + 1
	for i := shown; i < maxLines; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderPlanOverlay(w, h int) string {
	var b strings.Builder

	// Header (2 lines)
	header := titleStyle.Render(" teamoon v1.0.5 ")
	status := fmt.Sprintf("  %s Running    %s", runningDot, time.Now().Format("02 Jan 2006 15:04"))
	b.WriteString(header + status + "\n")
	titleBar := menuTitleStyle.Render(fmt.Sprintf(" Plan — Task #%d ", m.planTaskID))
	b.WriteString(titleBar + "\n")

	// Viewport: all remaining height minus header(2) + border(2) + help(1) + padding(1)
	maxShow := h - 6
	if maxShow < 5 {
		maxShow = 5
	}

	totalLines := len(m.planLines)
	scroll := m.planScroll

	// Clamp scroll
	maxScroll := totalLines - maxShow
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	endIdx := scroll + maxShow
	if endIdx > totalLines {
		endIdx = totalLines
	}

	// Render visible lines
	for i := scroll; i < endIdx; i++ {
		b.WriteString(m.planLines[i] + "\n")
	}
	// Pad to fill viewport
	for i := endIdx - scroll; i < maxShow; i++ {
		b.WriteString("\n")
	}

	// Help + position
	var pos string
	if totalLines > maxShow {
		pct := 0
		if maxScroll > 0 {
			pct = scroll * 100 / maxScroll
		}
		pos = fmt.Sprintf("  [%d%%] line %d/%d", pct, scroll+1, totalLines)
	} else {
		pos = fmt.Sprintf("  %d lines", totalLines)
	}
	b.WriteString(helpStyle.Render(" ↑↓/jk: scroll  pgup/pgdn: page  g/G: top/bottom  esc/q: close" + pos))

	return b.String()
}

func (m Model) renderDetailOverlay(w, h int) string {
	var b strings.Builder

	header := titleStyle.Render(" teamoon v1.0.5 ")
	status := fmt.Sprintf("  %s Running    %s", runningDot, time.Now().Format("02 Jan 2006 15:04"))
	b.WriteString(header + status + "\n")

	var desc string
	for _, t := range m.tasks {
		if t.ID == m.detailTaskID {
			desc = t.Description
			break
		}
	}
	titleBar := menuTitleStyle.Render(fmt.Sprintf(" Task #%d — %s ", m.detailTaskID, desc))
	b.WriteString(titleBar + "\n")

	maxShow := h - 6
	if maxShow < 5 {
		maxShow = 5
	}

	totalLines := len(m.detailLines)
	scroll := m.detailScroll

	maxScroll := totalLines - maxShow
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	endIdx := scroll + maxShow
	if endIdx > totalLines {
		endIdx = totalLines
	}

	for i := scroll; i < endIdx; i++ {
		line := m.detailLines[i]
		// Color-code log entries by level markers
		switch {
		case strings.Contains(line, "[ OK ]") || strings.Contains(line, "complete") || strings.Contains(line, "All steps"):
			b.WriteString(activeStyle.Render(line) + "\n")
		case strings.Contains(line, "[WARN]") || strings.Contains(line, "retry") || strings.Contains(line, "failed"):
			b.WriteString(medStyle.Render(line) + "\n")
		case strings.Contains(line, "[ERR ]") || strings.Contains(line, "FAILED:"):
			b.WriteString(staleStyle.Render(line) + "\n")
		case strings.HasPrefix(strings.TrimSpace(line), "──"):
			b.WriteString(mdDimStyle.Render(line) + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}
	for i := endIdx - scroll; i < maxShow; i++ {
		b.WriteString("\n")
	}

	var pos string
	if totalLines > maxShow {
		pct := 0
		if maxScroll > 0 {
			pct = scroll * 100 / maxScroll
		}
		pos = fmt.Sprintf("  [%d%%] line %d/%d", pct, scroll+1, totalLines)
	} else {
		pos = fmt.Sprintf("  %d lines", totalLines)
	}
	b.WriteString(helpStyle.Render(" ↑↓/jk: scroll  pgup/pgdn: page  g/G: top/bottom  esc/q: close" + pos))

	return b.String()
}

func (m Model) renderProjects(width int, maxItems int) string {
	var b strings.Builder
	title := "PROJECTS"
	if m.focus == "projects" {
		title = "» " + title
	}
	b.WriteString(panelTitleStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.projects))) + "\n")

	// Viewport scrolling
	startIdx, endIdx := viewport(m.projCursor, len(m.projects), maxItems)

	colName := 24
	colBranch := 14
	colMod := 8

	for i := startIdx; i < endIdx; i++ {
		p := m.projects[i]
		var icon, name string
		if !p.HasGit {
			icon = noGitStyle.Render("~")
			name = noGitStyle.Render(p.Name)
		} else if p.Active {
			icon = activeStyle.Render("✓")
			name = activeStyle.Render(p.Name)
		} else if p.Stale {
			icon = staleStyle.Render("✗")
			name = staleStyle.Render(p.Name)
		} else {
			icon = inactiveStyle.Render("○")
			name = inactiveStyle.Render(p.Name)
		}
		name = padRight(name, p.Name, colName)

		branch := p.Branch
		if branch == "" {
			branch = "—"
		}
		if len(branch) > colBranch {
			branch = branch[:colBranch-1] + "."
		}

		modified := "—"
		if p.Modified > 0 {
			modified = fmt.Sprintf("%d edit", p.Modified)
		}

		commit := p.LastCommit
		if commit == "" {
			commit = "—"
		}

		prefix := "  "
		if m.focus == "projects" && i == m.projCursor {
			prefix = "> "
		}

		line := prefix + icon + " " +
			name + " " +
			padStr(branch, colBranch) + " " +
			padStr(modified, colMod) + " " +
			commit

		if m.focus == "projects" && i == m.projCursor {
			b.WriteString(cursorStyle.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}

	if startIdx > 0 {
		b.WriteString(fmt.Sprintf("  ↑ %d more above\n", startIdx))
	}
	if endIdx < len(m.projects) {
		b.WriteString(fmt.Sprintf("  ↓ %d more below\n", len(m.projects)-endIdx))
	}

	return b.String()
}

func (m Model) renderMenuOverlay(w, h int) string {
	var b strings.Builder

	// Header
	header := titleStyle.Render(" teamoon v1.0.5 ")
	status := fmt.Sprintf("  %s Running    %s", runningDot, time.Now().Format("02 Jan 2006 15:04"))
	b.WriteString(header + status + "\n\n")

	if m.projCursor >= len(m.projects) {
		return b.String()
	}
	p := m.projects[m.projCursor]

	// Input mode — task creation
	if m.inputMode {
		b.WriteString(menuTitleStyle.Render(fmt.Sprintf(" %s — New Task ", p.Name)) + "\n\n")
		b.WriteString(fmt.Sprintf("  Task: %s_\n\n", m.inputBuffer))

		priorities := []string{"high", "med", "low"}
		b.WriteString("  Priority: ")
		for _, pri := range priorities {
			if pri == m.inputPriority {
				b.WriteString(cursorStyle.Render(fmt.Sprintf(" %s ", pri)))
			} else {
				b.WriteString(inactiveStyle.Render(fmt.Sprintf(" %s ", pri)))
			}
			b.WriteString("  ")
		}
		b.WriteString("\n")

		if m.menuStatus != "" {
			b.WriteString("\n  " + m.menuStatus + "\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render(" enter: save  esc: cancel  ↑/↓: priority"))
		return b.String()
	}

	// Menu title
	b.WriteString(menuTitleStyle.Render(fmt.Sprintf(" %s ", p.Name)) + "\n\n")

	// Dependabot option
	depCount := len(m.menuDepBot)
	if depCount > 0 {
		b.WriteString(menuOptionStyle.Render(fmt.Sprintf("  1. Merge dependabot PRs (%d)", depCount)) + "\n")
	} else {
		b.WriteString(inactiveStyle.Render("  1. Merge dependabot PRs (0)") + "\n")
	}

	// Git pull
	b.WriteString(menuOptionStyle.Render("  2. Git pull") + "\n")

	// Open PRs
	prCount := len(m.menuPRs)
	b.WriteString(menuOptionStyle.Render(fmt.Sprintf("  3. Open PRs (%d)", prCount)) + "\n")

	// Add task
	b.WriteString(menuOptionStyle.Render("  4. Add task") + "\n")

	b.WriteString("\n")

	// Show PR list if loaded
	if len(m.menuPRs) > 0 {
		b.WriteString(panelTitleStyle.Render("Open PRs") + "\n")
		for _, pr := range m.menuPRs {
			author := pr.Author.Login
			isDep := author == "app/dependabot" || author == "dependabot[bot]"
			line := fmt.Sprintf("  #%-5d %-50s %s → %s", pr.Number, pr.Title, author, pr.BaseRefName)
			if len(line) > w-4 {
				line = line[:w-7] + "..."
			}
			if isDep {
				b.WriteString(medStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Status
	if m.menuStatus != "" {
		b.WriteString("  " + m.menuStatus + "\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" 1: merge deps  2: pull  3: PRs  4: add task  esc/q: back"))

	return b.String()
}

func padRight(styled string, raw string, targetWidth int) string {
	visibleLen := len(raw)
	if visibleLen >= targetWidth {
		return styled
	}
	return styled + strings.Repeat(" ", targetWidth-visibleLen)
}

// viewport calculates visible window [start, end) given cursor, total items, and max visible.
func viewport(cursor, total, maxVisible int) (int, int) {
	if total <= maxVisible {
		return 0, total
	}
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	if start < 0 {
		start = 0
	}
	return start, end
}

// padStr pads a plain string (no ANSI) to targetWidth.
func padStr(s string, targetWidth int) string {
	if len(s) >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-len(s))
}

// padRaw pads a styled string using lipgloss.Width for visible width.
func padRaw(styled string, targetWidth int) string {
	vis := lipgloss.Width(styled)
	if vis >= targetWidth {
		return styled
	}
	return styled + strings.Repeat(" ", targetWidth-vis)
}

func formatNum(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatPriority(p string) string {
	switch strings.ToLower(p) {
	case "high":
		return highStyle.Render("HIGH")
	case "med":
		return medStyle.Render("MED ")
	case "low":
		return lowStyle.Render("LOW ")
	default:
		return lowStyle.Render("—   ")
	}
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "now"
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

var (
	mdH1Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("57")).Padding(0, 1)
	mdH2Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Underline(true)
	mdH3Style     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	mdBoldStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mdBulletStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	mdVerifyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Italic(true)
	mdTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mdDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func renderMarkdownLines(raw string, width int) []string {
	if width < 30 {
		width = 30
	}
	textW := width - 6
	if textW < 20 {
		textW = 20
	}
	sepW := width - 4
	if sepW > 50 {
		sepW = 50
	}

	var out []string
	lines := strings.Split(raw, "\n")
	prevEmpty := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			if !prevEmpty {
				out = append(out, "")
			}
			title := trimmed[4:]
			wrapped := wordWrap(title, textW-4)
			out = append(out, "  "+mdH3Style.Render("▸ "+wrapped[0]))
			for _, wl := range wrapped[1:] {
				out = append(out, "    "+mdH3Style.Render(wl))
			}
			out = append(out, "  "+mdDimStyle.Render(strings.Repeat("─", sepW)))
			prevEmpty = false

		case strings.HasPrefix(trimmed, "## "):
			out = append(out, "")
			title := trimmed[3:]
			wrapped := wordWrap(title, textW-4)
			for _, wl := range wrapped {
				out = append(out, "  "+mdH2Style.Render(wl))
			}
			out = append(out, "")
			prevEmpty = true

		case strings.HasPrefix(trimmed, "# "):
			title := trimmed[2:]
			wrapped := wordWrap(title, textW-4)
			out = append(out, mdH1Style.Render(" "+wrapped[0]+" "))
			for _, wl := range wrapped[1:] {
				out = append(out, mdH1Style.Render(" "+wl+" "))
			}
			out = append(out, "")
			prevEmpty = true

		case strings.HasPrefix(trimmed, "- **"):
			for _, wl := range wordWrap(trimmed[2:], textW-4) {
				out = append(out, "  "+mdBulletStyle.Render("●")+" "+styleBold(wl))
			}
			prevEmpty = false

		case strings.HasPrefix(trimmed, "- "):
			for _, wl := range wordWrap(trimmed[2:], textW-4) {
				out = append(out, "  "+mdBulletStyle.Render("●")+" "+mdTextStyle.Render(wl))
			}
			prevEmpty = false

		case strings.HasPrefix(strings.ToLower(trimmed), "verify:"):
			for _, wl := range wordWrap(trimmed, textW-6) {
				out = append(out, "    "+mdVerifyStyle.Render("✓ "+wl))
			}
			prevEmpty = false

		case trimmed == "":
			if !prevEmpty {
				out = append(out, "")
				prevEmpty = true
			}

		default:
			for _, wl := range wordWrap(trimmed, textW-4) {
				out = append(out, "    "+mdTextStyle.Render(styleBold(wl)))
			}
			prevEmpty = false
		}
	}
	return out
}

func styleBold(s string) string {
	result := s
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		bold := mdBoldStyle.Render(result[start+2 : end])
		result = result[:start] + bold + result[end+2:]
	}
	return result
}

func wordWrap(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
