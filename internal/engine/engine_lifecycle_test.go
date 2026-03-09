package engine

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/backend"
	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

// setupEngineEnv redirects HOME to a temp dir and creates the config dir.
func setupEngineEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dir := filepath.Join(tmp, ".config", "teamoon")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create Projects dir so projectinit doesn't fail
	if err := os.MkdirAll(filepath.Join(tmp, "Projects", "test-proj"), 0755); err != nil {
		t.Fatal(err)
	}
	return tmp
}

// msgCollector collects tea.Msg values sent by the engine.
type msgCollector struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (c *msgCollector) send(msg tea.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
}

func (c *msgCollector) stateMessages() []TaskStateMsg {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []TaskStateMsg
	for _, m := range c.msgs {
		if tsm, ok := m.(TaskStateMsg); ok {
			result = append(result, tsm)
		}
	}
	return result
}

func (c *msgCollector) lastState(taskID int) (queue.TaskState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.msgs) - 1; i >= 0; i-- {
		if tsm, ok := c.msgs[i].(TaskStateMsg); ok && tsm.TaskID == taskID {
			return tsm.State, true
		}
	}
	return "", false
}

func defaultTestConfig(home string) config.Config {
	cfg := config.DefaultConfig()
	cfg.ProjectsDir = filepath.Join(home, "Projects")
	cfg.Spawn.StepTimeoutMin = 0 // no timeout in tests
	return cfg
}

func singleStepPlan() plan.Plan {
	return plan.Plan{
		Steps: []plan.Step{
			{Number: 1, Title: "Implement feature", Body: "Write code", Agent: "dev"},
		},
	}
}

func multiStepPlan(n int) plan.Plan {
	steps := make([]plan.Step, n)
	for i := range steps {
		steps[i] = plan.Step{Number: i + 1, Title: "Step", Body: "Do work", Agent: "dev"}
	}
	return plan.Plan{Steps: steps}
}

// ---------- Manager.Start ----------

func TestManager_StartAndComplete(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write", "Read"},
	}}
	mgr := NewManager(b)

	task, err := queue.Add("test-proj", "test task", "high")
	if err != nil {
		t.Fatal(err)
	}

	col := &msgCollector{}
	mgr.Start(task, singleStepPlan(), cfg, col.send)

	// Wait for completion
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				// Verify final state
				state, ok := col.lastState(task.ID)
				if !ok {
					t.Fatal("no state messages received")
				}
				if state != queue.StateDone {
					t.Errorf("expected StateDone, got %s", state)
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestManager_DuplicateStart(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	// Use a backend that takes time to complete
	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "test task", "high")
	col := &msgCollector{}

	mgr.Start(task, multiStepPlan(3), cfg, col.send)
	// Second start should be no-op (runner already exists)
	mgr.Start(task, singleStepPlan(), cfg, col.send)

	// Wait for completion
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestManager_Stop(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	// Use a backend with many steps so we can stop mid-execution
	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "long task", "high")
	col := &msgCollector{}

	mgr.Start(task, multiStepPlan(10), cfg, col.send)

	// Give it a moment to start, then stop
	time.Sleep(100 * time.Millisecond)
	if mgr.IsRunning(task.ID) {
		mgr.Stop(task.ID)
	}

	if mgr.IsRunning(task.ID) {
		t.Error("task should not be running after Stop")
	}
}

func TestManager_IsRunning(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "test task", "high")

	if mgr.IsRunning(task.ID) {
		t.Error("task should not be running before Start")
	}

	col := &msgCollector{}
	mgr.Start(task, singleStepPlan(), cfg, col.send)

	// Wait for completion
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				return // success — it was running and is now done
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestManager_IsTaskRunningForProject(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "test task", "high")
	col := &msgCollector{}

	if mgr.IsTaskRunningForProject("test-proj") {
		t.Error("no task should be running initially")
	}

	mgr.Start(task, multiStepPlan(5), cfg, col.send)
	time.Sleep(50 * time.Millisecond)

	// May or may not still be running depending on NoopBackend speed
	// Just verify the method doesn't panic
	_ = mgr.IsTaskRunningForProject("test-proj")
	_ = mgr.IsTaskRunningForProject("other-proj")

	// Wait for completion
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				if mgr.IsTaskRunningForProject("test-proj") {
					t.Error("project should not be running after task completes")
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// ---------- runTask behavior ----------

func TestRunTask_AllStepsComplete(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Edit", "Bash"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "multi-step task", "high")
	col := &msgCollector{}

	mgr.Start(task, multiStepPlan(3), cfg, col.send)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				state, _ := col.lastState(task.ID)
				if state != queue.StateDone {
					t.Errorf("expected StateDone after all steps, got %s", state)
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestRunTask_StepFailure_MovesToDone(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode: 1, // always fail
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "failing task", "high")
	col := &msgCollector{}

	mgr.Start(task, singleStepPlan(), cfg, col.send)

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				// Forward-only: failed tasks move to done (not back to pending)
				state, _ := col.lastState(task.ID)
				if state != queue.StateDone {
					t.Errorf("expected StateDone after failure (forward-only), got %s", state)
				}
				// Verify fail reason is set
				updated, _ := queue.GetTask(task.ID)
				if updated.FailReason == "" {
					t.Error("expected FailReason to be set on failed task")
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestRunTask_NoWriteTools_Retries(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	// Exit 0 but only read tools — should retry
	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Read", "Glob"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "no-write task", "high")
	col := &msgCollector{}

	mgr.Start(task, singleStepPlan(), cfg, col.send)

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				// After maxRetries with no write tools, task should complete
				// on the last retry (retry check: retry < maxRetries-1)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestRunTask_ReadOnlyStep_NoRetry(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	// Exit 0, no write tools — but step is ReadOnly, so should succeed
	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Read", "Glob"},
	}}
	mgr := NewManager(b)

	task, _ := queue.Add("test-proj", "read-only task", "high")
	col := &msgCollector{}

	p := plan.Plan{
		Steps: []plan.Step{
			{Number: 1, Title: "Investigate", Body: "Read files", Agent: "analyst", ReadOnly: true},
		},
	}
	mgr.Start(task, p, cfg, col.send)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete within timeout")
		default:
			if !mgr.IsRunning(task.ID) {
				state, _ := col.lastState(task.ID)
				if state != queue.StateDone {
					t.Errorf("ReadOnly step should succeed without write tools, got %s", state)
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// ---------- Forward-only state guard ----------

func TestForwardOnlyState_RefusesBackward(t *testing.T) {
	setupEngineEnv(t)

	task, _ := queue.Add("test-proj", "guard test", "high")

	// pending → running (forward, OK)
	queue.UpdateState(task.ID, queue.StateRunning)
	got, _ := queue.GetTask(task.ID)
	if got.State != queue.StateRunning {
		t.Fatalf("expected running, got %s", got.State)
	}

	// running → pending (backward, REFUSED)
	queue.UpdateState(task.ID, queue.StatePending)
	got, _ = queue.GetTask(task.ID)
	if got.State != queue.StateRunning {
		t.Errorf("backward transition should be refused, got %s", got.State)
	}

	// running → done (forward, OK)
	queue.UpdateState(task.ID, queue.StateDone)
	got, _ = queue.GetTask(task.ID)
	if got.State != queue.StateDone {
		t.Errorf("expected done, got %s", got.State)
	}

	// done → pending (backward, REFUSED)
	queue.UpdateState(task.ID, queue.StatePending)
	got, _ = queue.GetTask(task.ID)
	if got.State != queue.StateDone {
		t.Errorf("done should never go backward, got %s", got.State)
	}
}

func TestForceUpdateState_AllowsBackward(t *testing.T) {
	setupEngineEnv(t)

	task, _ := queue.Add("test-proj", "force test", "high")

	queue.UpdateState(task.ID, queue.StateRunning)
	// ForceUpdateState bypasses guard
	queue.ForceUpdateState(task.ID, queue.StatePending)
	got, _ := queue.GetTask(task.ID)
	if got.State != queue.StatePending {
		t.Errorf("ForceUpdateState should allow backward, got %s", got.State)
	}
}

// ---------- Manager project loops ----------

func TestManager_StartProject_DuplicateReturnsFalse(t *testing.T) {
	b := &backend.NoopBackend{}
	mgr := NewManager(b)

	ok1 := mgr.StartProject("proj-a", func(ctx context.Context) {
		<-ctx.Done() // block until cancelled
	})
	if !ok1 {
		t.Fatal("first StartProject should return true")
	}
	defer mgr.StopProject("proj-a")

	ok2 := mgr.StartProject("proj-a", func(ctx context.Context) {
		<-ctx.Done()
	})
	if ok2 {
		t.Error("duplicate StartProject should return false")
	}
}

func TestManager_StartProject_UnlimitedProjects(t *testing.T) {
	b := &backend.NoopBackend{}
	mgr := NewManager(b)

	ok1 := mgr.StartProject("proj-a", func(ctx context.Context) {
		<-ctx.Done()
	})
	ok2 := mgr.StartProject("proj-b", func(ctx context.Context) {
		<-ctx.Done()
	})
	ok3 := mgr.StartProject("proj-c", func(ctx context.Context) {
		<-ctx.Done()
	})
	defer mgr.StopProject("proj-a")
	defer mgr.StopProject("proj-b")
	defer mgr.StopProject("proj-c")

	if !ok1 || !ok2 || !ok3 {
		t.Error("all projects should start — no project-loop limit")
	}
}

func TestManager_StopAll(t *testing.T) {
	home := setupEngineEnv(t)
	cfg := defaultTestConfig(home)

	b := &backend.NoopBackend{Result: backend.SpawnResult{
		ExitCode:  0,
		ToolsUsed: []string{"Write"},
	}}
	mgr := NewManager(b)

	t1, _ := queue.Add("test-proj", "task 1", "high")
	t2, _ := queue.Add("test-proj", "task 2", "high")
	col := &msgCollector{}

	mgr.Start(t1, multiStepPlan(5), cfg, col.send)
	mgr.Start(t2, multiStepPlan(5), cfg, col.send)

	time.Sleep(50 * time.Millisecond)
	mgr.StopAll()

	if mgr.IsRunning(t1.ID) || mgr.IsRunning(t2.ID) {
		t.Error("no tasks should be running after StopAll")
	}
}
