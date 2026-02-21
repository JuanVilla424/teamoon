package web

import (
	"os"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/metrics"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/projects"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

var Version, BuildNum string

type WebTask struct {
	queue.Task
	EffectiveState string `json:"effective_state"`
	IsRunning      bool   `json:"is_running"`
	HasPlan        bool   `json:"has_plan"`
}

type WebProject struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	LastCommit string `json:"last_commit"`
	Modified   int    `json:"modified"`
	Active     bool   `json:"active"`
	Stale      bool   `json:"stale"`
	HasGit     bool   `json:"has_git"`
	GitHubRepo string `json:"github_repo"`
	StatusIcon string `json:"status_icon"`
}

type LogEntryJSON struct {
	Time    time.Time `json:"time"`
	TaskID  int       `json:"task_id"`
	Project string    `json:"project"`
	Message string    `json:"message"`
	Level   string    `json:"level"`
}

type DataSnapshot struct {
	Timestamp  time.Time             `json:"timestamp"`
	Today      metrics.TokenSummary  `json:"today"`
	Week       metrics.TokenSummary  `json:"week"`
	Month      metrics.TokenSummary  `json:"month"`
	Cost       metrics.CostSummary   `json:"cost"`
	Session    metrics.SessionContext `json:"session"`
	Tasks      []WebTask             `json:"tasks"`
	Projects   []WebProject          `json:"projects"`
	LogEntries []LogEntryJSON        `json:"log_entries"`
	PlanModel  string                `json:"plan_model"`
	ExecModel  string                `json:"exec_model"`
	Effort     string                `json:"effort"`
	Version    string                `json:"version"`
	BuildNum   string                `json:"build_num"`
}

type Store struct {
	mu        sync.RWMutex
	snapshot  DataSnapshot
	logBuf    *logs.RingBuffer
	engineMgr *engine.Manager
	cfg       config.Config
}

func NewStore(cfg config.Config, mgr *engine.Manager, logBuf *logs.RingBuffer) *Store {
	return &Store{
		cfg:       cfg,
		engineMgr: mgr,
		logBuf:    logBuf,
	}
}

func (s *Store) Refresh() {
	today, week, month, _ := metrics.ScanTokens(s.cfg.ClaudeDir)
	session := metrics.ScanActiveSession(s.cfg.ClaudeDir, s.cfg.ContextLimit)
	cost := metrics.CalculateCost(today, week, month, s.cfg)
	projs := projects.Scan(s.cfg.ProjectsDir)
	activeTasks, _ := queue.ListActive()

	webTasks := make([]WebTask, len(activeTasks))
	for i, t := range activeTasks {
		effState := string(queue.EffectiveState(t))
		// Show "generating" if plan generation is in progress
		generatingMu.Lock()
		if effState == "pending" && generatingSet[t.ID] {
			effState = "generating"
		}
		generatingMu.Unlock()
		webTasks[i] = WebTask{
			Task:           t,
			EffectiveState: effState,
			IsRunning:      s.engineMgr.IsRunning(t.ID),
			HasPlan:        plan.PlanExists(t.ID),
		}
	}

	webProjects := make([]WebProject, len(projs))
	for i, p := range projs {
		icon := "inactive"
		if !p.HasGit {
			icon = "no_git"
		} else if p.Active {
			icon = "active"
		} else if p.Stale {
			icon = "stale"
		}
		webProjects[i] = WebProject{
			Name:       p.Name,
			Path:       p.Path,
			Branch:     p.Branch,
			LastCommit: p.LastCommit,
			Modified:   p.Modified,
			Active:     p.Active,
			Stale:      p.Stale,
			HasGit:     p.HasGit,
			GitHubRepo: p.GitHubRepo,
			StatusIcon: icon,
		}
	}

	entries := s.logBuf.Snapshot()
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

	planModel, execModel := parsePlanExecModel(os.Getenv("CLAUDE_CODE_MODEL"))

	snap := DataSnapshot{
		Timestamp:  time.Now(),
		Today:      today,
		Week:       week,
		Month:      month,
		Cost:       cost,
		Session:    session,
		Tasks:      webTasks,
		Projects:   webProjects,
		LogEntries: logJSON,
		PlanModel:  planModel,
		ExecModel:  execModel,
		Effort:     os.Getenv("CLAUDE_CODE_EFFORT_LEVEL"),
		Version:    Version,
		BuildNum:   BuildNum,
	}

	s.mu.Lock()
	s.snapshot = snap
	s.mu.Unlock()
}

func (s *Store) Get() DataSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func parsePlanExecModel(env string) (string, string) {
	switch env {
	case "opusplan":
		return "Opus 4.6", "Sonnet 4.6"
	case "opus":
		return "Opus 4.6", "Opus 4.6"
	case "sonnet":
		return "Sonnet 4.6", "Sonnet 4.6"
	case "haiku":
		return "Haiku 4.5", "Haiku 4.5"
	default:
		return "Opus 4.6", "Sonnet 4.6"
	}
}
