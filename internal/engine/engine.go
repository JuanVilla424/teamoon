package engine

import (
	"context"
	"os/exec"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	taskID int
	cancel context.CancelFunc
	done   chan struct{}
}

type Manager struct {
	mu      sync.Mutex
	runners map[int]*Runner
}

func NewManager() *Manager {
	return &Manager{runners: make(map[int]*Runner)}
}

func (m *Manager) Start(task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg)) {
	m.mu.Lock()
	if _, exists := m.runners[task.ID]; exists {
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{
		taskID: task.ID,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.runners[task.ID] = r
	m.mu.Unlock()

	if _, err := exec.LookPath("claude"); err != nil {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  task.ID,
			Project: task.Project,
			Message: "claude CLI not found in PATH",
			Level:   logs.LevelError,
		}})
		send(TaskStateMsg{TaskID: task.ID, State: queue.StateBlocked, Message: "claude CLI not found"})
		close(r.done)
		return
	}

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.runners, task.ID)
			m.mu.Unlock()
			close(r.done)
		}()
		runTask(ctx, task, p, cfg, send)
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
