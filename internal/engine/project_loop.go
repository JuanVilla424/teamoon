package engine

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

// PlanFunc generates a plan for a task synchronously.
// logFn is called with tool names as planning progresses (for real-time visibility).
type PlanFunc func(t queue.Task, sk config.SkeletonConfig, logFn func(string)) (plan.Plan, error)

// planBackoffSchedule maps attempt index to wait duration before that attempt.
var planBackoffSchedule = []time.Duration{
	0,                // attempt 1: no wait
	30 * time.Second, // attempt 2: 30s
	2 * time.Minute,  // attempt 3: 2m
	5 * time.Minute,  // attempt 4+: 5m
}

func planBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if attempt < len(planBackoffSchedule) {
		return planBackoffSchedule[attempt]
	}
	return planBackoffSchedule[len(planBackoffSchedule)-1]
}

func effectiveMaxPlanAttempts(cfg config.Config) int {
	if cfg.Spawn.MaxPlanAttempts > 0 {
		return cfg.Spawn.MaxPlanAttempts
	}
	return 3
}

// firstEligibleTask returns the first task not exhausted by plan attempts.
// Returns nil if all tasks are exhausted.
func firstEligibleTask(tasks []queue.Task, maxAttempts int, emit func(logs.LogLevel, string)) *queue.Task {
	for i := range tasks {
		t := &tasks[i]
		state := queue.EffectiveState(*t)
		if state == queue.StatePending && t.PlanAttempts >= maxAttempts {
			emit(logs.LevelWarn, fmt.Sprintf(
				"Task #%d exhausted plan attempts (%d/%d), skipping — use Reset to retry",
				t.ID, t.PlanAttempts, maxAttempts,
			))
			continue
		}
		return t
	}
	return nil
}

// groupByWave groups tasks by wave number. Wave 0 tasks are treated as sequential (each in its own group).
func groupByWave(tasks []queue.Task) [][]queue.Task {
	waveMap := make(map[int][]queue.Task)
	var seqTasks []queue.Task
	for _, t := range tasks {
		if t.Wave == 0 {
			seqTasks = append(seqTasks, t)
		} else {
			waveMap[t.Wave] = append(waveMap[t.Wave], t)
		}
	}

	// Collect wave numbers and sort
	var waveNums []int
	for w := range waveMap {
		waveNums = append(waveNums, w)
	}
	sort.Ints(waveNums)

	var groups [][]queue.Task
	for _, w := range waveNums {
		groups = append(groups, waveMap[w])
	}
	// Sequential tasks go at the end, one per group
	for _, t := range seqTasks {
		groups = append(groups, []queue.Task{t})
	}
	return groups
}

// planOneTask plans a single task. Returns the plan or an error.
func planOneTask(ctx context.Context, task queue.Task, cfg config.Config, planFn PlanFunc, send func(tea.Msg), emit func(logs.LogLevel, string)) (plan.Plan, bool) {
	maxAttempts := effectiveMaxPlanAttempts(cfg)
	skeleton := config.SkeletonFor(cfg, task.Project)

	backoff := planBackoff(task.PlanAttempts)
	if backoff > 0 {
		emit(logs.LevelWarn, fmt.Sprintf(
			"Task #%d plan retry %d/%d, backing off %s...",
			task.ID, task.PlanAttempts+1, maxAttempts, backoff,
		))
		select {
		case <-ctx.Done():
			return plan.Plan{}, false
		case <-time.After(backoff):
		}
	}

	emit(logs.LevelInfo, fmt.Sprintf("Planning task #%d (attempt %d/%d): %s",
		task.ID, task.PlanAttempts+1, maxAttempts, task.Description))
	send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: "planning"})
	planLogFn := func(toolName string) {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			TaskID:  task.ID,
			Project: task.Project,
			Message: "Planning: " + toolName,
			Level:   logs.LevelInfo,
		}})
	}
	p, planErr := planFn(task, skeleton, planLogFn)
	if planErr != nil {
		attempts, _ := queue.IncrementPlanAttempts(task.ID)
		reason := fmt.Sprintf("plan generation failed (attempt %d/%d): %v", attempts, maxAttempts, planErr)
		emit(logs.LevelError, fmt.Sprintf("Plan failed for task #%d: %v", task.ID, planErr))
		queue.SetFailReason(task.ID, reason)
		send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: "plan_failed"})
		return plan.Plan{}, false
	}
	emit(logs.LevelSuccess, fmt.Sprintf("Plan ready for task #%d", task.ID))
	send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned})
	return p, true
}

// RunProjectLoop processes autopilot-eligible tasks for a project with wave-aware execution.
// Tasks with the same wave number run in parallel. Different waves run sequentially.
// Tasks with wave=0 (legacy) run sequentially after all waved tasks.
func RunProjectLoop(ctx context.Context, project string, cfg config.Config, planFn PlanFunc, send func(tea.Msg), mgr *Manager) {
	emit := func(level logs.LogLevel, msg string) {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			Project: project,
			Message: msg,
			Level:   level,
		}})
	}

	emit(logs.LevelInfo, fmt.Sprintf("Project autopilot started for %s", project))

	for {
		if ctx.Err() != nil {
			emit(logs.LevelWarn, fmt.Sprintf("Project autopilot stopped for %s", project))
			return
		}

		tasks, err := queue.ListAutopilotPending(project)
		if err != nil {
			emit(logs.LevelError, fmt.Sprintf("Failed to list tasks: %v", err))
			return
		}
		if len(tasks) == 0 {
			emit(logs.LevelSuccess, fmt.Sprintf("No more autopilot tasks for %s", project))
			return
		}

		maxAttempts := effectiveMaxPlanAttempts(cfg)

		// Filter eligible tasks
		var eligible []queue.Task
		for _, t := range tasks {
			state := queue.EffectiveState(t)
			if state == queue.StatePending && t.PlanAttempts >= maxAttempts {
				emit(logs.LevelWarn, fmt.Sprintf(
					"Task #%d exhausted plan attempts (%d/%d), skipping",
					t.ID, t.PlanAttempts, maxAttempts,
				))
				continue
			}
			eligible = append(eligible, t)
		}
		if len(eligible) == 0 {
			emit(logs.LevelSuccess, fmt.Sprintf("No more eligible autopilot tasks for %s", project))
			return
		}

		// Group by wave and process wave by wave
		waves := groupByWave(eligible)
		if len(waves) == 0 {
			return
		}

		// Process only the first wave group, then re-scan
		wave := waves[0]

		for reason := CheckGuardrails(); reason != ""; reason = CheckGuardrails() {
			emit(logs.LevelWarn, fmt.Sprintf("Guardrail: %s, waiting 2m...", reason))
			select {
			case <-ctx.Done():
				emit(logs.LevelWarn, fmt.Sprintf("Project autopilot stopped for %s", project))
				return
			case <-time.After(2 * time.Minute):
			}
		}

		if len(wave) == 1 {
			// Single task — same sequential behavior as before
			task := wave[0]
			if cfg.Debug {
				log.Printf("[debug][autopilot] selected task #%d (wave %d) state=%s", task.ID, task.Wave, queue.EffectiveState(task))
			}

			state := queue.EffectiveState(task)
			if state == queue.StatePending {
				p, ok := planOneTask(ctx, task, cfg, planFn, send, emit)
				if !ok {
					if ctx.Err() != nil {
						return
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				runOneTask(ctx, task, p, cfg, send, mgr, emit)
			} else if state == queue.StatePlanned {
				p, parseErr := plan.ParsePlan(plan.PlanPath(task.ID))
				if parseErr != nil {
					emit(logs.LevelError, fmt.Sprintf("Plan parse failed for task #%d: %v", task.ID, parseErr))
					queue.SetFailReason(task.ID, "plan parse failed: "+parseErr.Error())
					continue
				}
				runOneTask(ctx, task, p, cfg, send, mgr, emit)
			}
		} else {
			// Multi-task wave — plan sequentially, then run in parallel
			waveNum := wave[0].Wave
			var taskIDs []int
			for _, t := range wave {
				taskIDs = append(taskIDs, t.ID)
			}
			emit(logs.LevelInfo, fmt.Sprintf("Wave %d: %d tasks %v — planning sequentially", waveNum, len(wave), taskIDs))

			type taskPlan struct {
				task queue.Task
				plan plan.Plan
			}
			var planned []taskPlan

			for _, task := range wave {
				if ctx.Err() != nil {
					return
				}
				state := queue.EffectiveState(task)
				if state == queue.StatePending {
					p, ok := planOneTask(ctx, task, cfg, planFn, send, emit)
					if !ok {
						continue // skip failed plans, run the rest
					}
					planned = append(planned, taskPlan{task: task, plan: p})
				} else if state == queue.StatePlanned {
					p, parseErr := plan.ParsePlan(plan.PlanPath(task.ID))
					if parseErr != nil {
						emit(logs.LevelError, fmt.Sprintf("Plan parse failed for task #%d: %v", task.ID, parseErr))
						queue.SetFailReason(task.ID, "plan parse failed: "+parseErr.Error())
						continue
					}
					planned = append(planned, taskPlan{task: task, plan: p})
				}
			}

			if len(planned) == 0 {
				emit(logs.LevelWarn, fmt.Sprintf("Wave %d: no tasks planned successfully, skipping", waveNum))
				continue
			}

			emit(logs.LevelInfo, fmt.Sprintf("Wave %d: running %d tasks in parallel", waveNum, len(planned)))

			var wg sync.WaitGroup
			for _, tp := range planned {
				wg.Add(1)
				go func(t queue.Task, p plan.Plan) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
					}
					runOneTask(ctx, t, p, cfg, send, mgr, emit)
				}(tp.task, tp.plan)
			}
			wg.Wait()

			emit(logs.LevelSuccess, fmt.Sprintf("Wave %d completed (%d tasks)", waveNum, len(planned)))
		}

		// Small delay between waves/tasks
		select {
		case <-ctx.Done():
			emit(logs.LevelWarn, fmt.Sprintf("Project autopilot stopped for %s", project))
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// RunSystemLoop processes system-assignee tasks sequentially.
func RunSystemLoop(ctx context.Context, cfg config.Config, planFn PlanFunc, send func(tea.Msg), mgr *Manager) {
	emit := func(level logs.LogLevel, msg string) {
		send(LogMsg{Entry: logs.LogEntry{
			Time:    time.Now(),
			Project: "_system",
			Message: msg,
			Level:   level,
		}})
	}

	emit(logs.LevelInfo, "System executor started")

	for {
		if ctx.Err() != nil {
			emit(logs.LevelWarn, "System executor stopped")
			return
		}

		tasks, err := queue.ListAutopilotSystemPending()
		if err != nil {
			emit(logs.LevelError, fmt.Sprintf("Failed to list system tasks: %v", err))
			return
		}
		if len(tasks) == 0 {
			emit(logs.LevelSuccess, "No more system tasks")
			return
		}

		maxAttempts := effectiveMaxPlanAttempts(cfg)
		taskPtr := firstEligibleTask(tasks, maxAttempts, emit)
		if taskPtr == nil {
			emit(logs.LevelSuccess, "No more eligible system tasks (some exhausted plan attempts)")
			return
		}
		task := *taskPtr
		state := queue.EffectiveState(task)

		for reason := CheckGuardrails(); reason != ""; reason = CheckGuardrails() {
			emit(logs.LevelWarn, fmt.Sprintf("Guardrail: %s, waiting 2m...", reason))
			select {
			case <-ctx.Done():
				emit(logs.LevelWarn, "System executor stopped")
				return
			case <-time.After(2 * time.Minute):
			}
		}

		skeleton := cfg.Skeleton

		if state == queue.StatePending {
			// Apply backoff based on previous failures
			backoff := planBackoff(task.PlanAttempts)
			if backoff > 0 {
				emit(logs.LevelWarn, fmt.Sprintf(
					"System task #%d plan retry %d/%d, backing off %s...",
					task.ID, task.PlanAttempts+1, maxAttempts, backoff,
				))
				select {
				case <-ctx.Done():
					emit(logs.LevelWarn, "System executor stopped")
					return
				case <-time.After(backoff):
				}
			}

			emit(logs.LevelInfo, fmt.Sprintf("Planning system task #%d (attempt %d/%d): %s",
				task.ID, task.PlanAttempts+1, maxAttempts, task.Description))
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: "planning"})
			sysLogFn := func(toolName string) {
				send(LogMsg{Entry: logs.LogEntry{
					Time:    time.Now(),
					TaskID:  task.ID,
					Project: task.Project,
					Message: "Planning: " + toolName,
					Level:   logs.LevelInfo,
				}})
			}
			p, planErr := planFn(task, skeleton, sysLogFn)
			if planErr != nil {
				attempts, _ := queue.IncrementPlanAttempts(task.ID)
				reason := fmt.Sprintf("plan generation failed (attempt %d/%d): %v", attempts, maxAttempts, planErr)
				emit(logs.LevelError, fmt.Sprintf("Plan failed for system task #%d: %v", task.ID, planErr))
				queue.SetFailReason(task.ID, reason)
				send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: "plan_failed"})
				continue
			}
			emit(logs.LevelSuccess, fmt.Sprintf("Plan ready for system task #%d", task.ID))
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned})
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			runOneTask(ctx, task, p, cfg, send, mgr, emit)
		} else if state == queue.StatePlanned {
			p, parseErr := plan.ParsePlan(plan.PlanPath(task.ID))
			if parseErr != nil {
				emit(logs.LevelError, fmt.Sprintf("Plan parse failed for system task #%d: %v", task.ID, parseErr))
				queue.SetFailReason(task.ID, "plan parse failed: "+parseErr.Error())
				continue
			}
			runOneTask(ctx, task, p, cfg, send, mgr, emit)
		}

		select {
		case <-ctx.Done():
			emit(logs.LevelWarn, "System executor stopped")
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// runOneTask starts a single task via the engine manager and waits for completion or cancellation.
func runOneTask(ctx context.Context, task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg), mgr *Manager, emit func(logs.LogLevel, string)) {
	// Acquire concurrency slot (blocks if max concurrent reached)
	mgr.AcquireSlot()
	defer mgr.ReleaseSlot()

	taskDone := make(chan struct{})

	// Wrap send to detect task completion
	wrappedSend := func(msg tea.Msg) {
		send(msg)
		if tsm, ok := msg.(TaskStateMsg); ok {
			if tsm.TaskID == task.ID && (tsm.State == queue.StateDone || tsm.State == queue.StatePending) {
				select {
				case taskDone <- struct{}{}:
				default:
				}
			}
		}
	}

	queue.UpdateState(task.ID, queue.StateRunning)
	mgr.Start(task, p, cfg, wrappedSend)
	emit(logs.LevelInfo, fmt.Sprintf("Running task #%d", task.ID))

	select {
	case <-ctx.Done():
		return // loop exits; task keeps running on its own
	case <-taskDone:
		// Task finished (done or back to pending), continue loop
	}
}
