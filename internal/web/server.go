package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/jobs"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/metrics"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/plangen"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

type sseClient chan []byte

type Hub struct {
	mu      sync.Mutex
	clients map[sseClient]struct{}
}

func newHub() *Hub {
	return &Hub{clients: make(map[sseClient]struct{})}
}

func (h *Hub) register(c sseClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c sseClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) broadcast(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- data:
		default:
		}
	}
}

type Server struct {
	cfg            config.Config
	store          *Store
	hub            *Hub
	sessions       *sessionStore
	refreshMu      sync.Mutex
	refreshPending bool
}

func NewServer(cfg config.Config, mgr *engine.Manager, logBuf *logs.RingBuffer) *Server {
	store := NewStore(cfg, mgr, logBuf)
	hub := newHub()
	return &Server{cfg: cfg, store: store, hub: hub, sessions: newSessionStore()}
}

func (s *Server) RecoverAndResume() {
	noopEmit := func(level logs.LogLevel, msg string) {
		log.Printf("[recovery] %s", msg)
	}

	// Phase 1: Reset tasks without session, then direct-resume those with a plan
	recovered, err := queue.RecoverRunning()
	if err != nil {
		log.Printf("[recovery] error: %v", err)
		return
	}
	for _, t := range recovered {
		log.Printf("[recovery] task #%d (%s) reset to %s (step %d)", t.ID, t.Project, t.State, t.CurrentStep)
		if t.PlanFile != "" {
			p, parseErr := plan.ParsePlan(plan.PlanPath(t.ID))
			if parseErr != nil {
				log.Printf("[recovery] task #%d plan parse failed, staying %s: %v", t.ID, t.State, parseErr)
				continue
			}
			// Acquire slot in goroutine so recovery doesn't bypass concurrency limits
			go func(task queue.Task, p plan.Plan) {
				s.store.engineMgr.AcquireSlot()
				defer s.store.engineMgr.ReleaseSlot()
				engine.StartAndWaitTask(context.Background(), task, p, s.cfg, s.webSend(task.ID), s.store.engineMgr, noopEmit)
			}(t, p)
			log.Printf("[recovery] direct resume task #%d (%s) from step %d (slot-gated)", t.ID, t.Project, t.CurrentStep)
		}
	}

	// Phase 2: Resume tasks with session_id (can continue where they left off)
	resumable, err := queue.ListResumable()
	if err != nil {
		log.Printf("[recovery] error listing resumable tasks: %v", err)
	}
	for _, t := range resumable {
		p, parseErr := plan.ParsePlan(plan.PlanPath(t.ID))
		if parseErr != nil {
			log.Printf("[recovery] task #%d plan parse failed, falling back to planned: %v", t.ID, parseErr)
			queue.ForceUpdateState(t.ID, queue.StatePlanned)
			continue
		}
		// Clear stale session ID — runTask already skips completed steps
		// via step.Number <= task.CurrentStep, so session resume is unnecessary
		// and can hang if the session expired.
		queue.SetSessionID(t.ID, "")
		// Acquire slot in goroutine so recovery doesn't bypass concurrency limits
		go func(task queue.Task, p plan.Plan) {
			s.store.engineMgr.AcquireSlot()
			defer s.store.engineMgr.ReleaseSlot()
			engine.StartAndWaitTask(context.Background(), task, p, s.cfg, s.webSend(task.ID), s.store.engineMgr, noopEmit)
		}(t, p)
		log.Printf("[recovery] resuming task #%d (%s) from step %d (slot-gated, session cleared)",
			t.ID, t.Project, t.CurrentStep)
	}

	if !s.cfg.AutopilotAutostart {
		log.Printf("[recovery] autopilot autostart disabled, skipping resume")
		return
	}

	// Phase 3: Autopilot autostart
	projects, err := queue.AutopilotProjects()
	if err != nil {
		log.Printf("[recovery] error listing projects: %v", err)
		return
	}
	for _, project := range projects {
		cfg := s.cfg
		send := s.webSend(0)
		ok := s.store.engineMgr.StartProject(project, func(ctx context.Context) {
			engine.RunProjectLoop(ctx, project, cfg, func(t queue.Task, sk config.SkeletonConfig, logFn func(string)) (plan.Plan, error) {
				return plangen.GeneratePlan(t, sk, cfg, logFn)
			}, send, s.store.engineMgr)
		})
		if ok {
			log.Printf("[recovery] autopilot resumed for project: %s", project)
		}
	}

	if sysTasks, _ := queue.ListAutopilotSystemPending(); len(sysTasks) > 0 {
		s.startSystemLoop()
		log.Printf("[recovery] system executor resumed with %d tasks", len(sysTasks))
	}
}

// startAutopilotForNewProjects starts autopilot loops for projects that have
// autopilot-eligible tasks but no running loop (e.g. after harvester creates tasks).
func (s *Server) startAutopilotForNewProjects(ctx context.Context) {
	projects, err := queue.AutopilotProjects()
	if err != nil {
		return
	}
	cfg := s.cfg
	send := s.webSend(0)
	for _, project := range projects {
		if s.store.engineMgr.IsProjectRunning(project) {
			continue
		}
		s.store.engineMgr.StartProject(project, func(ctx context.Context) {
			engine.RunProjectLoop(ctx, project, cfg, func(t queue.Task, sk config.SkeletonConfig, logFn func(string)) (plan.Plan, error) {
				return plangen.GeneratePlan(t, sk, cfg, logFn)
			}, send, s.store.engineMgr)
		})
		log.Printf("[harvester] auto-started autopilot for project: %s", project)
	}
}

func (s *Server) startSystemLoop() {
	cfg := s.cfg
	send := s.webSend(0)
	s.store.engineMgr.StartProject("_system", func(ctx context.Context) {
		engine.RunSystemLoop(ctx, cfg, func(t queue.Task, sk config.SkeletonConfig, logFn func(string)) (plan.Plan, error) {
			return plangen.GeneratePlan(t, sk, cfg, logFn)
		}, send, s.store.engineMgr)
	})
}

func (s *Server) Start(ctx context.Context) {
	metrics.StartUsageFetcher(s.cfg.ProjectsDir)
	s.store.Refresh()
	s.RecoverAndResume()
	s.startAutopilotForNewProjects(ctx)

	// Seed default jobs (harvester, etc.) and start scheduler
	jobs.SeedDefaults()
	jobCfg := s.cfg
	jobs.StartScheduler(ctx, func(j jobs.Job) {
		jobs.RunJob(ctx, j, jobCfg)
		// After harvester creates security tasks, start autopilot loops for new projects
		if j.Instruction == "__harvester__" {
			s.startAutopilotForNewProjects(ctx)
		}
		s.refreshAndBroadcast()
	})

	go func() {
		interval := time.Duration(s.cfg.RefreshIntervalSec) * time.Second
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.store.Refresh()
				snap := s.store.Get()
				data, err := json.Marshal(snap)
				if err == nil {
					s.hub.broadcast(data)
				}
			}
		}
	}()

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(staticFS())))

	// Onboarding routes — no auth (password may not be set yet)
	mux.HandleFunc("/api/onboarding/status", s.logRequest(s.handleOnboardingStatus))
	mux.HandleFunc("/api/onboarding/prereqs", s.logRequest(s.handleOnboardingPrereqs))
	mux.HandleFunc("/api/onboarding/prereqs/install", s.logRequest(s.handleOnboardingPrereqsInstall))
	mux.HandleFunc("/api/onboarding/config", s.logRequest(s.handleOnboardingConfig))
	mux.HandleFunc("/api/onboarding/skills", s.logRequest(s.handleOnboardingSkills))
	mux.HandleFunc("/api/onboarding/bmad", s.logRequest(s.handleOnboardingBMAD))
	mux.HandleFunc("/api/onboarding/hooks", s.logRequest(s.handleOnboardingHooks))
	mux.HandleFunc("/api/onboarding/mcp", s.logRequest(s.handleOnboardingMCP))
	mux.HandleFunc("/api/onboarding/mcp/optional", s.logRequest(s.handleOnboardingMCPOptional))
	mux.HandleFunc("/api/onboarding/plugins", s.logRequest(s.handleOnboardingPlugins))

	mux.HandleFunc("/api/auth/login", s.logRequest(s.handleLogin))
	mux.HandleFunc("/api/auth/logout", s.logRequest(s.handleLogout))

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/data", s.authWrap(s.handleData))
	mux.HandleFunc("/api/sse", s.authWrap(s.handleSSE))
	mux.HandleFunc("/api/tasks/add", s.logRequest(s.authWrap(s.handleTaskAdd)))
	mux.HandleFunc("/api/tasks/done", s.logRequest(s.authWrap(s.handleTaskDone)))
	mux.HandleFunc("/api/tasks/archive", s.logRequest(s.authWrap(s.handleTaskArchive)))
	mux.HandleFunc("/api/tasks/bulk-archive", s.logRequest(s.authWrap(s.handleTaskBulkArchive)))
	mux.HandleFunc("/api/tasks/replan", s.logRequest(s.authWrap(s.handleTaskReplan)))
	mux.HandleFunc("/api/tasks/autopilot", s.logRequest(s.authWrap(s.handleTaskAutopilot)))
	mux.HandleFunc("/api/tasks/stop", s.logRequest(s.authWrap(s.handleTaskStop)))
	mux.HandleFunc("/api/tasks/plan", s.logRequest(s.authWrap(s.handleTaskPlan)))
	mux.HandleFunc("/api/tasks/detail", s.logRequest(s.authWrap(s.handleTaskDetail)))
	mux.HandleFunc("/api/projects/prs", s.logRequest(s.authWrap(s.handleProjectPRs)))
	mux.HandleFunc("/api/projects/pr-detail", s.logRequest(s.authWrap(s.handleProjectPRDetail)))
	mux.HandleFunc("/api/projects/merge-dependabot", s.logRequest(s.authWrap(s.handleMergeDependabot)))
	mux.HandleFunc("/api/projects/pull", s.logRequest(s.authWrap(s.handleProjectPull)))
	mux.HandleFunc("/api/projects/git-init", s.logRequest(s.authWrap(s.handleProjectGitInit)))
	mux.HandleFunc("/api/projects/autopilot/start", s.logRequest(s.authWrap(s.handleProjectAutopilotStart)))
	mux.HandleFunc("/api/projects/autopilot/stop", s.logRequest(s.authWrap(s.handleProjectAutopilotStop)))
	mux.HandleFunc("/api/projects/skeleton", s.logRequest(s.authWrap(s.handleProjectSkeleton)))
	mux.HandleFunc("/api/templates/list", s.logRequest(s.authWrap(s.handleTemplateList)))
	mux.HandleFunc("/api/templates/add", s.logRequest(s.authWrap(s.handleTemplateAdd)))
	mux.HandleFunc("/api/templates/delete", s.logRequest(s.authWrap(s.handleTemplateDelete)))
	mux.HandleFunc("/api/templates/update", s.logRequest(s.authWrap(s.handleTemplateUpdate)))
	mux.HandleFunc("/api/tasks/assignee", s.logRequest(s.authWrap(s.handleTaskAssignee)))
	mux.HandleFunc("/api/tasks/update", s.logRequest(s.authWrap(s.handleTaskUpdate)))
	mux.HandleFunc("/api/chat/send", s.logRequest(s.authWrap(s.handleChatSend)))
	mux.HandleFunc("/api/chat/history", s.logRequest(s.authWrap(s.handleChatHistory)))
	mux.HandleFunc("/api/chat/clear", s.logRequest(s.authWrap(s.handleChatClear)))
	mux.HandleFunc("/api/projects/init", s.logRequest(s.authWrap(s.handleProjectInit)))
	mux.HandleFunc("/api/config", s.logRequest(s.authWrap(s.handleConfigGet)))
	mux.HandleFunc("/api/config/save", s.logRequest(s.authWrap(s.handleConfigSave)))
	mux.HandleFunc("/api/mcp/optional", s.logRequest(s.authWrap(s.handleMCPOptionalList)))
	mux.HandleFunc("/api/mcp/list", s.logRequest(s.authWrap(s.handleMCPList)))
	mux.HandleFunc("/api/mcp/toggle", s.logRequest(s.authWrap(s.handleMCPToggle)))
	mux.HandleFunc("/api/mcp/init", s.logRequest(s.authWrap(s.handleMCPInit)))
	mux.HandleFunc("/api/mcp/catalog", s.logRequest(s.authWrap(s.handleMCPCatalog)))
	mux.HandleFunc("/api/mcp/install", s.logRequest(s.authWrap(s.handleMCPInstall)))
	mux.HandleFunc("/api/mcp/uninstall", s.logRequest(s.authWrap(s.handleMCPUninstall)))
	mux.HandleFunc("/api/plugins/list", s.logRequest(s.authWrap(s.handlePluginList)))
	mux.HandleFunc("/api/plugins/install", s.logRequest(s.authWrap(s.handlePluginInstall)))
	mux.HandleFunc("/api/plugins/uninstall", s.logRequest(s.authWrap(s.handlePluginUninstall)))
	mux.HandleFunc("/api/skills/list", s.logRequest(s.authWrap(s.handleSkillsList)))
	mux.HandleFunc("/api/skills/catalog", s.logRequest(s.authWrap(s.handleSkillsCatalog)))
	mux.HandleFunc("/api/skills/install", s.logRequest(s.authWrap(s.handleSkillsInstall)))
	mux.HandleFunc("/api/skills/uninstall", s.logRequest(s.authWrap(s.handleSkillsUninstall)))
	mux.HandleFunc("/api/jobs/list", s.logRequest(s.authWrap(s.handleJobsList)))
	mux.HandleFunc("/api/jobs/add", s.logRequest(s.authWrap(s.handleJobAdd)))
	mux.HandleFunc("/api/jobs/update", s.logRequest(s.authWrap(s.handleJobUpdate)))
	mux.HandleFunc("/api/jobs/delete", s.logRequest(s.authWrap(s.handleJobDelete)))
	mux.HandleFunc("/api/jobs/run", s.logRequest(s.authWrap(s.handleJobRun)))
	mux.HandleFunc("/api/update/check", s.logRequest(s.authWrap(s.handleUpdateCheck)))
	mux.HandleFunc("/api/update", s.logRequest(s.authWrap(s.handleUpdate)))
	mux.HandleFunc("/api/upload", s.logRequest(s.authWrap(s.handleUpload)))
	mux.HandleFunc("/api/uploads/", s.logRequest(s.authWrap(s.handleUploadServe)))
	mux.HandleFunc("/api/tasks/attach", s.logRequest(s.authWrap(s.handleTaskAttach)))

	addr := fmt.Sprintf("%s:%d", s.cfg.WebHost, s.cfg.WebPort)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx)
	}()

	log.Printf("[web] listening on http://%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[web] server error: %v", err)
	}
}

func (s *Server) logRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[http] %s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
	}
}

func (s *Server) authWrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.WebPassword == "" {
			next(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.sessions.validate(cookie.Value) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	content, err := staticContent("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Write(content)
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snap := s.store.Get()
	json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	client := make(sseClient, 32)
	s.hub.register(client)
	defer s.hub.unregister(client)

	snap := s.store.Get()
	if data, err := json.Marshal(snap); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-client:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) refreshAndBroadcast() {
	queue.FlushIfDirty()
	s.store.Refresh()
	snap := s.store.Get()
	data, err := json.Marshal(snap)
	if err == nil {
		s.hub.broadcast(data)
	}
}

// scheduleRefresh batches rapid log-driven refreshes (max one per 200ms).
func (s *Server) scheduleRefresh() {
	s.refreshMu.Lock()
	if s.refreshPending {
		s.refreshMu.Unlock()
		return
	}
	s.refreshPending = true
	s.refreshMu.Unlock()

	time.AfterFunc(200*time.Millisecond, func() {
		s.refreshMu.Lock()
		s.refreshPending = false
		s.refreshMu.Unlock()
		s.refreshAndBroadcast()
	})
}
