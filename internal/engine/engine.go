package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/backend"
	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

type TaskStateMsg struct {
	TaskID  int
	State   queue.TaskState
	Message string
}

type LogMsg struct {
	Entry logs.LogEntry
}

type PlanGeneratedMsg struct {
	TaskID  int
	Content string
	Err     error
}

type Runner struct {
	taskID  int
	project string
	cancel  context.CancelFunc
	done    chan struct{}
}

type ProjectLoop struct {
	project string
	cancel  context.CancelFunc
	done    chan struct{}
}

type Manager struct {
	mu           sync.Mutex
	runners      map[int]*Runner
	projectLoops map[string]*ProjectLoop
	taskSem      chan struct{} // limits total concurrent CLI processes
	backend      backend.Backend
}

// NewManager creates a Manager with the given backend.
// If b is nil, the default backend (Claude) is used.
func NewManager(b backend.Backend) *Manager {
	if b == nil {
		b = backend.Default()
	}
	return &Manager{
		runners:      make(map[int]*Runner),
		projectLoops: make(map[string]*ProjectLoop),
		backend:      b,
	}
}

// Backend returns the manager's active backend.
func (m *Manager) Backend() backend.Backend { return m.backend }

// SetMaxConcurrentTasks configures the global task concurrency semaphore.
func (m *Manager) SetMaxConcurrentTasks(n int) {
	if n <= 0 {
		n = 3
	}
	m.taskSem = make(chan struct{}, n)
}

// AcquireSlot blocks until a task execution slot is available.
func (m *Manager) AcquireSlot() {
	if m.taskSem != nil {
		m.taskSem <- struct{}{}
	}
}

// ReleaseSlot returns a task execution slot.
func (m *Manager) ReleaseSlot() {
	if m.taskSem != nil {
		<-m.taskSem
	}
}

func (m *Manager) Start(task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg)) {
	m.mu.Lock()
	if _, exists := m.runners[task.ID]; exists {
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{
		taskID:  task.ID,
		project: task.Project,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	m.runners[task.ID] = r
	m.mu.Unlock()

	if err := m.backend.Available(); err != nil {
		msg := fmt.Sprintf("%s CLI not found in PATH", m.backend.Name())
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  task.ID,
			Project: task.Project,
			Message: msg,
			Level:   logs.LevelError,
		}})
		queue.SetFailReason(task.ID, msg)
		queue.UpdateState(task.ID, queue.StateDone)
		send(TaskStateMsg{TaskID: task.ID, State: queue.StateDone, Message: msg})
		close(r.done)
		return
	}

	b := m.backend
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.runners, task.ID)
			m.mu.Unlock()
			close(r.done)
		}()
		runTask(ctx, task, p, cfg, send, b)
	}()
}

func (m *Manager) Stop(taskID int) {
	m.mu.Lock()
	r, exists := m.runners[taskID]
	m.mu.Unlock()

	if !exists {
		return
	}

	r.cancel()
	<-r.done
}

func (m *Manager) IsRunning(taskID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.runners[taskID]
	return exists
}

// IsTaskRunningForProject returns true if any task is currently executing for the given project.
func (m *Manager) IsTaskRunningForProject(project string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runners {
		if r.project == project {
			return true
		}
	}
	return false
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	runners := make([]*Runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	for _, r := range runners {
		r.cancel()
		<-r.done
	}
}

// StartProject launches an autopilot loop for the given project.
// runLoop is called in a goroutine with the loop context.
// Returns false if already running for this project.
// No project-loop limit — taskSem controls real task concurrency.
func (m *Manager) StartProject(project string, runLoop func(ctx context.Context)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projectLoops[project]; exists {
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	pl := &ProjectLoop{
		project: project,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	m.projectLoops[project] = pl

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.projectLoops, project)
			m.mu.Unlock()
			close(pl.done)
		}()
		runLoop(ctx)
	}()

	return true
}

// StopProject cancels the project autopilot loop and waits for it to finish.
func (m *Manager) StopProject(project string) {
	m.mu.Lock()
	pl, exists := m.projectLoops[project]
	m.mu.Unlock()
	if !exists {
		return
	}
	pl.cancel()
	<-pl.done
}

// IsProjectRunning returns true if a project autopilot loop is active.
func (m *Manager) IsProjectRunning(project string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.projectLoops[project]
	return exists
}

// ActiveProjectLoops returns the names of all currently running project loops.
func (m *Manager) ActiveProjectLoops() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.projectLoops))
	for name := range m.projectLoops {
		names = append(names, name)
	}
	return names
}
