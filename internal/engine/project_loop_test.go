package engine

import (
	"testing"

	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

// ---------- groupByWave ----------

func TestGroupByWave_SingleWave(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, Wave: 1},
		{ID: 2, Wave: 1},
		{ID: 3, Wave: 1},
	}
	groups := groupByWave(tasks)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 3 {
		t.Errorf("expected 3 tasks in group, got %d", len(groups[0]))
	}
}

func TestGroupByWave_MultipleWaves(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, Wave: 2},
		{ID: 2, Wave: 1},
		{ID: 3, Wave: 3},
	}
	groups := groupByWave(tasks)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	// Should be sorted by wave number
	if groups[0][0].Wave != 1 {
		t.Errorf("first group should be wave 1, got %d", groups[0][0].Wave)
	}
	if groups[1][0].Wave != 2 {
		t.Errorf("second group should be wave 2, got %d", groups[1][0].Wave)
	}
	if groups[2][0].Wave != 3 {
		t.Errorf("third group should be wave 3, got %d", groups[2][0].Wave)
	}
}

func TestGroupByWave_ZeroWave(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, Wave: 0},
		{ID: 2, Wave: 0},
		{ID: 3, Wave: 0},
	}
	groups := groupByWave(tasks)
	// Wave 0 tasks each go in their own group
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (one per wave-0 task), got %d", len(groups))
	}
	for i, g := range groups {
		if len(g) != 1 {
			t.Errorf("group %d should have 1 task, got %d", i, len(g))
		}
	}
}

func TestGroupByWave_Mixed(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, Wave: 1},
		{ID: 2, Wave: 1},
		{ID: 3, Wave: 0}, // sequential — goes at end
		{ID: 4, Wave: 2},
	}
	groups := groupByWave(tasks)
	// wave 1 (2 tasks) + wave 2 (1 task) + wave 0 (1 task sequential) = 3 groups
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	// First group: wave 1
	if len(groups[0]) != 2 {
		t.Errorf("wave 1 group should have 2 tasks, got %d", len(groups[0]))
	}
	// Second group: wave 2
	if len(groups[1]) != 1 || groups[1][0].ID != 4 {
		t.Error("wave 2 group should have task 4")
	}
	// Third group: wave 0 sequential
	if len(groups[2]) != 1 || groups[2][0].ID != 3 {
		t.Error("last group should be sequential wave-0 task 3")
	}
}

func TestGroupByWave_Empty(t *testing.T) {
	groups := groupByWave(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

// ---------- firstEligibleTask ----------

func noopEmit(_ logs.LogLevel, _ string) {}

func TestFirstEligibleTask_AllEligible(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, State: queue.StatePending, PlanAttempts: 0},
		{ID: 2, State: queue.StatePending, PlanAttempts: 0},
	}
	result := firstEligibleTask(tasks, 3, noopEmit)
	if result == nil {
		t.Fatal("expected a task, got nil")
	}
	if result.ID != 1 {
		t.Errorf("expected task 1, got %d", result.ID)
	}
}

func TestFirstEligibleTask_FirstExhausted(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, State: queue.StatePending, PlanAttempts: 3},
		{ID: 2, State: queue.StatePending, PlanAttempts: 1},
	}
	result := firstEligibleTask(tasks, 3, noopEmit)
	if result == nil {
		t.Fatal("expected a task, got nil")
	}
	if result.ID != 2 {
		t.Errorf("expected task 2 (skip exhausted), got %d", result.ID)
	}
}

func TestFirstEligibleTask_AllExhausted(t *testing.T) {
	tasks := []queue.Task{
		{ID: 1, State: queue.StatePending, PlanAttempts: 3},
		{ID: 2, State: queue.StatePending, PlanAttempts: 5},
	}
	result := firstEligibleTask(tasks, 3, noopEmit)
	if result != nil {
		t.Errorf("expected nil when all exhausted, got task %d", result.ID)
	}
}

func TestFirstEligibleTask_PlannedNotExhausted(t *testing.T) {
	// Planned tasks pass through regardless of PlanAttempts because
	// the exhaustion check only applies to pending state.
	tasks := []queue.Task{
		{ID: 1, State: queue.StatePlanned, PlanAttempts: 5},
	}
	result := firstEligibleTask(tasks, 3, noopEmit)
	if result == nil {
		t.Fatal("planned task should not be skipped by exhaustion check")
	}
}

// ---------- planBackoff ----------

func TestPlanBackoff_Schedule(t *testing.T) {
	if planBackoff(0) != 0 {
		t.Error("attempt 0 should have no backoff")
	}
	if planBackoff(1) != planBackoffSchedule[1] {
		t.Error("attempt 1 should use schedule[1]")
	}
	if planBackoff(100) != planBackoffSchedule[len(planBackoffSchedule)-1] {
		t.Error("large attempt should use last schedule entry")
	}
}
