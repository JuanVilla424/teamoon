package dashboard

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/metrics"
	"github.com/JuanVilla424/teamoon/internal/projects"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

type tickMsg time.Time

type Model struct {
	cfg        config.Config
	today      metrics.TokenSummary
	week       metrics.TokenSummary
	month      metrics.TokenSummary
	cost       metrics.CostSummary
	session    metrics.SessionContext
	projects   []projects.Project
	tasks      []queue.Task
	planModel  string
	execModel  string
	effort     string
	focus      string
	cursor     int
	projCursor int
	showMenu   bool
	menuPRs       []projects.PR
	menuDepBot    []projects.PR
	menuStatus    string
	inputMode     bool
	inputBuffer   string
	inputPriority string
	width      int
	height     int
	ready      bool
	err        error

	// Autopilot engine
	engineMgr      *engine.Manager
	msgChan        chan tea.Msg
	logBuf         *logs.RingBuffer
	logEntries     []logs.LogEntry

	// Plan overlay
	showPlan     bool
	planContent  string
	planLines    []string
	planTaskID   int
	planScroll   int

	// Generating plan indicator
	generatingPlan   bool
	generatingTaskID int

	// Detail overlay
	showDetail   bool
	detailLines  []string
	detailTaskID int
	detailScroll int
}

func NewModel(cfg config.Config, mgr *engine.Manager, logBuf *logs.RingBuffer) Model {
	plan, exec := parseModelConfig(os.Getenv("CLAUDE_CODE_MODEL"))
	return Model{
		cfg:       cfg,
		planModel: plan,
		execModel: exec,
		effort:    os.Getenv("CLAUDE_CODE_EFFORT_LEVEL"),
		focus:     "projects",
		engineMgr: mgr,
		msgChan:   make(chan tea.Msg, 64),
		logBuf:    logBuf,
	}
}

func parseModelConfig(env string) (string, string) {
	switch env {
	case "opusplan":
		return "Opus 4.6", "Sonnet 4.6"
	case "opus":
		return "Opus 4.6", "Opus 4.6"
	case "sonnet":
		return "Sonnet 4.6", "Sonnet 4.6"
	case "haiku":
		return "Haiku 4.5", "Haiku 4.5"
	case "":
		return "Opus 4.6", "Sonnet 4.6"
	default:
		return env, env
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		refreshData,
		tickEvery(time.Duration(m.cfg.RefreshIntervalSec)*time.Second),
		listenChannel(m.msgChan),
	)
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refreshData() tea.Msg {
	return refreshMsg{}
}

func listenChannel(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

type refreshMsg struct{}

type dataMsg struct {
	today    metrics.TokenSummary
	week     metrics.TokenSummary
	month    metrics.TokenSummary
	session  metrics.SessionContext
	projects []projects.Project
	tasks    []queue.Task
	err      error
}

type prInfoMsg struct {
	prs    []projects.PR
	depBot []projects.PR
	err    error
}

type mergeMsg struct {
	merged int
	failed int
	err    error
}

type pullMsg struct {
	output string
	err    error
}

type taskAddMsg struct{ err error }

func fetchData(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		today, week, month, err := metrics.ScanTokens(cfg.ClaudeDir)
		session := metrics.ScanActiveSession(cfg.ClaudeDir, cfg.ContextLimit)
		projs := projects.Scan(cfg.ProjectsDir)
		tasks, _ := queue.ListActive()
		return dataMsg{
			today:    today,
			week:     week,
			month:    month,
			session:  session,
			projects: projs,
			tasks:    tasks,
			err:      err,
		}
	}
}
