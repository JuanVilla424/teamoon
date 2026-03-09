package web

import (
	"os"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/jobs"
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
	Name             string `json:"name"`
	Path             string `json:"path"`
	Branch           string `json:"branch"`
	LastCommit       string `json:"last_commit"`
	Modified         int    `json:"modified"`
	Active           bool   `json:"active"`
	Stale            bool   `json:"stale"`
	HasGit           bool   `json:"has_git"`
	GitHubRepo       string `json:"github_repo"`
	StatusIcon       string `json:"status_icon"`
	AutopilotRunning bool   `json:"autopilot_running"`
	TaskTotal        int    `json:"task_total"`
	TaskPending      int    `json:"task_pending"`
	TaskRunning      int    `json:"task_running"`
	TaskDone         int    `json:"task_done"`
}

type LogEntryJSON struct {
	Time    time.Time `json:"time"`
	TaskID  int       `json:"task_id"`
	Project string    `json:"project"`
	Message string    `json:"message"`
	Level   string    `json:"level"`
	Agent   string    `json:"agent,omitempty"`
}

type DataSnapshot struct {
	Timestamp  time.Time             `json:"timestamp"`
	Today      metrics.TokenSummary  `json:"today"`
	Week       metrics.TokenSummary  `json:"week"`
	Month      metrics.TokenSummary  `json:"month"`
	Cost       metrics.CostSummary   `json:"cost"`
	Session    metrics.SessionContext `json:"session"`
	Usage      metrics.ClaudeUsage   `json:"usage"`
	Tasks      []WebTask             `json:"tasks"`
	Projects   []WebProject          `json:"projects"`
	LogEntries []LogEntryJSON        `json:"log_entries"`
	PlanModel         string   `json:"plan_model"`
	ExecModel         string   `json:"exec_model"`
	Effort            string   `json:"effort"`
	Version           string   `json:"version"`
	BuildNum          string   `json:"build_num"`
	ProjectAutopilots []string  `json:"project_autopilots"`
	UptimeSec         int64    `json:"uptime_sec"`
	AuthEnabled       bool     `json:"auth_enabled"`
	Jobs              []jobs.Job `json:"jobs"`
}

type Store struct {
	mu        sync.RWMutex
	snapshot  DataSnapshot
	logBuf    *logs.RingBuffer
	engineMgr *engine.Manager
	cfg       config.Config
	startTime time.Time

	// TTL cache for expensive scans
	tokensCachedAt   time.Time
	tokensToday      metrics.TokenSummary
	tokensWeek       metrics.TokenSummary
	tokensMonth      metrics.TokenSummary
	sessionCachedAt  time.Time
	sessionResult    metrics.SessionContext
	projectsCachedAt time.Time
	projectsResult   []projects.Project
}

func NewStore(cfg config.Config, mgr *engine.Manager, logBuf *logs.RingBuffer) *Store {
	return &Store{
		cfg:       cfg,
		engineMgr: mgr,
		logBuf:    logBuf,
		startTime: time.Now(),
	}
}

func (s *Store) Refresh() {
	now := time.Now()

	// ScanTokens: 2-minute TTL (walks 7K+ JSONL files)
	if now.Sub(s.tokensCachedAt) >= 2*time.Minute {
		t, w, m, _ := metrics.ScanTokens(s.cfg.ClaudeDir)
		s.tokensToday, s.tokensWeek, s.tokensMonth = t, w, m
		s.tokensCachedAt = now
	}

	// ScanActiveSession: 2-minute TTL (walks same JSONL files)
	if now.Sub(s.sessionCachedAt) >= 2*time.Minute {
		s.sessionResult = metrics.ScanActiveSession(s.cfg.ClaudeDir, s.cfg.ContextLimit)
		s.sessionCachedAt = now
	}

	// projects.Scan: 1-minute TTL (spawns git subprocesses per project)
	if now.Sub(s.projectsCachedAt) >= 1*time.Minute {
		s.projectsResult = projects.Scan(s.cfg.ProjectsDir)
		s.projectsCachedAt = now
	}

	today, week, month := s.tokensToday, s.tokensWeek, s.tokensMonth
	session := s.sessionResult
	cost := metrics.CalculateCost(today, week, month)
	usage := metrics.GetUsage()
	projs := s.projectsResult
	activeTasks, _ := queue.ListActive()

	// Snapshot generating set once to avoid locking per-task
	generatingMu.Lock()
	genSnapshot := make(map[int]bool, len(generatingSet))
	for id := range generatingSet {
		genSnapshot[id] = true
	}
	generatingMu.Unlock()

	webTasks := make([]WebTask, len(activeTasks))
	for i, t := range activeTasks {
		effState := string(queue.EffectiveState(t))
		isRunning := s.engineMgr.IsRunning(t.ID)
		if effState == "pending" && genSnapshot[t.ID] {
			effState = "generating"
		}
		// Engine is authoritative: if running, override stale JSON state
		if isRunning && (effState == "pending" || effState == "planned") {
			effState = "running"
		}
		// Task was running (has step progress) but briefly went planned during gap — keep as running
		if !isRunning && effState == "planned" && t.CurrentStep > 0 && s.engineMgr.IsProjectRunning(t.Project) {
			effState = "running"
		}
		webTasks[i] = WebTask{
			Task:           t,
			EffectiveState: effState,
			IsRunning:      isRunning,
			HasPlan:        plan.PlanExists(t.ID),
		}
	}

	// Count tasks per project
	type projCounts struct{ total, pending, running, done int }
	projTaskCounts := make(map[string]*projCounts)
	for _, wt := range webTasks {
		pc := projTaskCounts[wt.Project]
		if pc == nil {
			pc = &projCounts{}
			projTaskCounts[wt.Project] = pc
		}
		pc.total++
		switch wt.EffectiveState {
		case "pending", "generating", "planned":
			pc.pending++
		case "running":
			pc.running++
		case "done":
			pc.done++
		}
	}

	activeLoops := s.engineMgr.ActiveProjectLoops()
	activeLoopSet := make(map[string]bool, len(activeLoops))
	for _, name := range activeLoops {
		activeLoopSet[name] = true
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
		pc := projTaskCounts[p.Name]
		wp := WebProject{
			Name:             p.Name,
			Path:             p.Path,
			Branch:           p.Branch,
			LastCommit:       p.LastCommit,
			Modified:         p.Modified,
			Active:           p.Active,
			Stale:            p.Stale,
			HasGit:           p.HasGit,
			GitHubRepo:       p.GitHubRepo,
			StatusIcon:       icon,
			AutopilotRunning: activeLoopSet[p.Name],
		}
		if pc != nil {
			wp.TaskTotal = pc.total
			wp.TaskPending = pc.pending
			wp.TaskRunning = pc.running
			wp.TaskDone = pc.done
		}
		webProjects[i] = wp
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
			Agent:   e.Agent,
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
		Usage:      usage,
		Tasks:      webTasks,
		Projects:   webProjects,
		LogEntries: logJSON,
		PlanModel:  planModel,
		ExecModel:  execModel,
		Effort:     os.Getenv("CLAUDE_CODE_EFFORT_LEVEL"),
		Version:           Version,
		BuildNum:           BuildNum,
		ProjectAutopilots: activeLoops,
		UptimeSec:         int64(time.Since(s.startTime).Seconds()),
		AuthEnabled:       s.cfg.WebPassword != "",
	}
	if jl, err := jobs.ListAll(); err == nil {
		snap.Jobs = jl
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
