package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

// setupTestEnv redirects HOME to a temp dir so config.ConfigDir() resolves
// to <tmp>/.config/teamoon. t.Setenv implicitly forbids t.Parallel(), which
// is correct because all queue functions share the package-level storeMu.
func setupTestEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dir := filepath.Join(tmp, ".config", "teamoon")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ---------- CRUD ----------

func TestAdd(t *testing.T) {
	setupTestEnv(t)

	before := time.Now()
	task, err := Add("myproj", "do stuff", "high")
	if err != nil {
		t.Fatal(err)
	}

	if task.ID != 1 {
		t.Errorf("expected ID=1, got %d", task.ID)
	}
	if task.State != StatePending {
		t.Errorf("expected state pending, got %s", task.State)
	}
	if task.Project != "myproj" {
		t.Errorf("expected project myproj, got %s", task.Project)
	}
	if task.Description != "do stuff" {
		t.Errorf("expected description 'do stuff', got %s", task.Description)
	}
	if task.Priority != "high" {
		t.Errorf("expected priority high, got %s", task.Priority)
	}
	if task.CreatedAt.Before(before) || task.CreatedAt.After(time.Now()) {
		t.Errorf("CreatedAt %v not in expected range", task.CreatedAt)
	}
}

func TestAdd_DefaultPriority(t *testing.T) {
	setupTestEnv(t)

	task, err := Add("proj", "desc", "")
	if err != nil {
		t.Fatal(err)
	}
	if task.Priority != "med" {
		t.Errorf("expected default priority 'med', got %s", task.Priority)
	}
}

func TestAdd_MultipleIncrementNextID(t *testing.T) {
	setupTestEnv(t)

	for i := 1; i <= 3; i++ {
		task, err := Add("proj", "task", "low")
		if err != nil {
			t.Fatal(err)
		}
		if task.ID != i {
			t.Errorf("expected ID=%d, got %d", i, task.ID)
		}
	}
}

func TestMarkDone(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "high")
	if err := MarkDone(task.ID); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 task, got %d", len(all))
	}
	if all[0].State != StateDone {
		t.Errorf("expected state done, got %s", all[0].State)
	}
	if !all[0].Done {
		t.Error("expected Done=true")
	}
}

func TestMarkDone_NotFound(t *testing.T) {
	setupTestEnv(t)

	err := MarkDone(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestArchive(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	if err := Archive(task.ID); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if all[0].State != StateArchived {
		t.Errorf("expected state archived, got %s", all[0].State)
	}
	if !all[0].Done {
		t.Error("expected Done=true after archive")
	}
}

func TestArchive_NotFound(t *testing.T) {
	setupTestEnv(t)

	err := Archive(999)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

// ---------- Update operations ----------

func TestUpdateState(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")

	for _, state := range []TaskState{StatePlanned, StateRunning, StateFailed, StateDone} {
		if err := UpdateState(task.ID, state); err != nil {
			t.Fatalf("UpdateState(%s): %v", state, err)
		}
		all, _ := ListAll()
		if all[0].State != state {
			t.Errorf("expected state %s, got %s", state, all[0].State)
		}
	}
}

func TestUpdateState_DoneSetsBool(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	if err := UpdateState(task.ID, StateDone); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if !all[0].Done {
		t.Error("expected Done=true when state set to done")
	}
}

func TestUpdateState_NotFound(t *testing.T) {
	setupTestEnv(t)

	err := UpdateState(999, StatePlanned)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestSetPlanFile(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	if err := SetPlanFile(task.ID, "/tmp/plan.md"); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if all[0].PlanFile != "/tmp/plan.md" {
		t.Errorf("expected PlanFile=/tmp/plan.md, got %s", all[0].PlanFile)
	}
	if all[0].State != StatePlanned {
		t.Errorf("expected state planned, got %s", all[0].State)
	}
}

func TestResetPlan(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	_ = SetPlanFile(task.ID, "/tmp/plan.md")
	_ = SetFailReason(task.ID, "some reason")

	if err := ResetPlan(task.ID); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if all[0].State != StatePending {
		t.Errorf("expected state pending after reset, got %s", all[0].State)
	}
	if all[0].PlanFile != "" {
		t.Errorf("expected empty PlanFile, got %s", all[0].PlanFile)
	}
	if all[0].FailReason != "" {
		t.Errorf("expected empty FailReason, got %s", all[0].FailReason)
	}
	if all[0].Done {
		t.Error("expected Done=false after reset")
	}
}

func TestSetFailReason(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	if err := SetFailReason(task.ID, "build failed"); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if all[0].FailReason != "build failed" {
		t.Errorf("expected 'build failed', got %s", all[0].FailReason)
	}
	if all[0].State != StateFailed {
		t.Errorf("expected state failed, got %s", all[0].State)
	}
}

func TestUpdateAssignee(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med")
	if err := UpdateAssignee(task.ID, "claude-agent"); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	if all[0].Assignee != "claude-agent" {
		t.Errorf("expected assignee 'claude-agent', got %s", all[0].Assignee)
	}
}

func TestToggleAutoPilot(t *testing.T) {
	setupTestEnv(t)

	task, _ := Add("proj", "desc", "med") // AutoPilot defaults false

	if err := ToggleAutoPilot(task.ID); err != nil {
		t.Fatal(err)
	}
	all, _ := ListAll()
	if !all[0].AutoPilot {
		t.Error("expected AutoPilot=true after first toggle")
	}

	if err := ToggleAutoPilot(task.ID); err != nil {
		t.Fatal(err)
	}
	all, _ = ListAll()
	if all[0].AutoPilot {
		t.Error("expected AutoPilot=false after second toggle")
	}
}

func TestSetAllAutoPilot(t *testing.T) {
	setupTestEnv(t)

	t1, _ := Add("proj", "task1", "med")
	t2, _ := Add("proj", "task2", "med")
	t3, _ := Add("proj", "task3", "med")
	_ = MarkDone(t3.ID) // done task should be unchanged

	if err := SetAllAutoPilot(true); err != nil {
		t.Fatal(err)
	}

	all, _ := ListAll()
	for _, task := range all {
		switch task.ID {
		case t1.ID, t2.ID:
			if !task.AutoPilot {
				t.Errorf("task #%d: expected AutoPilot=true", task.ID)
			}
		case t3.ID:
			if task.AutoPilot {
				t.Error("done task should not have AutoPilot toggled")
			}
		}
	}
}

// ---------- List/filter ----------

func TestListPending(t *testing.T) {
	setupTestEnv(t)

	Add("proj", "t1", "med")                    // pending
	t2, _ := Add("proj", "t2", "med")           // will be done
	t3, _ := Add("proj", "t3", "med")           // will be archived
	_ = MarkDone(t2.ID)
	_ = Archive(t3.ID)

	pending, err := ListPending()
	if err != nil {
		t.Fatal(err)
	}
	// ListPending excludes StateDone only; archived has State="archived" so it passes the filter
	if len(pending) != 2 {
		t.Errorf("expected 2 non-done tasks, got %d", len(pending))
	}
}

func TestListActive(t *testing.T) {
	setupTestEnv(t)

	Add("proj", "t1", "med")                   // pending
	t2, _ := Add("proj", "t2", "med")          // will be done
	t3, _ := Add("proj", "t3", "med")          // will be archived
	_ = MarkDone(t2.ID)
	_ = Archive(t3.ID)

	active, err := ListActive()
	if err != nil {
		t.Fatal(err)
	}
	// ListActive excludes archived only
	if len(active) != 2 {
		t.Errorf("expected 2 active tasks, got %d", len(active))
	}
}

func TestListAll(t *testing.T) {
	setupTestEnv(t)

	Add("proj", "t1", "med")
	t2, _ := Add("proj", "t2", "med")
	t3, _ := Add("proj", "t3", "med")
	_ = MarkDone(t2.ID)
	_ = Archive(t3.ID)

	all, err := ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(all))
	}
}

func TestEffectiveState(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want TaskState
	}{
		{"empty state not done", Task{State: "", Done: false}, StatePending},
		{"empty state done", Task{State: "", Done: true}, StateDone},
		{"explicit running", Task{State: StateRunning}, StateRunning},
		{"explicit failed", Task{State: StateFailed}, StateFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveState(tt.task)
			if got != tt.want {
				t.Errorf("EffectiveState(%+v) = %s, want %s", tt.task, got, tt.want)
			}
		})
	}
}

// ---------- Persistence ----------

func TestPersistence_WriteReadCycle(t *testing.T) {
	cfgDir := setupTestEnv(t)

	Add("proj1", "desc1", "high")
	Add("proj2", "desc2", "low")

	data, err := os.ReadFile(filepath.Join(cfgDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}

	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}

	if store.NextID != 3 {
		t.Errorf("expected NextID=3, got %d", store.NextID)
	}
	if len(store.Tasks) != 2 {
		t.Fatalf("expected 2 tasks on disk, got %d", len(store.Tasks))
	}
	if store.Tasks[0].ID != 1 || store.Tasks[1].ID != 2 {
		t.Errorf("unexpected task IDs: %d, %d", store.Tasks[0].ID, store.Tasks[1].ID)
	}
	if store.Tasks[0].Project != "proj1" || store.Tasks[1].Project != "proj2" {
		t.Error("project data mismatch on disk")
	}
}

func TestNextID_PersistenceAcrossLoads(t *testing.T) {
	setupTestEnv(t)

	Add("proj", "t1", "med") // ID=1
	Add("proj", "t2", "med") // ID=2

	// Third add should get ID=3 (NextID survived load/save cycle)
	task, err := Add("proj", "t3", "med")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 3 {
		t.Errorf("expected ID=3, got %d", task.ID)
	}
}

// ---------- Edge cases ----------

func TestCorruptFile(t *testing.T) {
	cfgDir := setupTestEnv(t)

	if err := os.WriteFile(filepath.Join(cfgDir, "tasks.json"), []byte("{{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Add("proj", "desc", "med")
	if err == nil {
		t.Fatal("expected error for corrupt json, got nil")
	}
}

func TestUnreadableFile(t *testing.T) {
	cfgDir := setupTestEnv(t)

	fp := filepath.Join(cfgDir, "tasks.json")
	if err := os.WriteFile(fp, []byte(`{"next_id":1,"tasks":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fp, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(fp, 0644) // restore so TempDir cleanup succeeds
	})

	_, err := Add("proj", "desc", "med")
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestEmptyStore_ListsReturnEmpty(t *testing.T) {
	setupTestEnv(t)

	all, err := ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(all))
	}

	pending, err := ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}

	active, err := ListActive()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active, got %d", len(active))
	}
}

func TestNoFileYet_AddCreatesFile(t *testing.T) {
	cfgDir := setupTestEnv(t)

	fp := filepath.Join(cfgDir, "tasks.json")

	// Verify file does not exist yet
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Fatal("tasks.json should not exist before first Add")
	}

	if _, err := Add("proj", "first task", "med"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(fp); err != nil {
		t.Errorf("tasks.json should exist after Add, got error: %v", err)
	}
}

// Verify that config.ConfigDir() resolves through our test env
func TestConfigDirUsesHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got := config.ConfigDir()
	want := filepath.Join(tmp, ".config", "teamoon")
	if got != want {
		t.Errorf("ConfigDir() = %s, want %s", got, want)
	}
}
