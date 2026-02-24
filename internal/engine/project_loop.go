package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

// PlanFunc generates a plan for a task synchronously.
type PlanFunc func(t queue.Task, sk config.SkeletonConfig) (plan.Plan, error)

// RunProjectLoop processes autopilot-eligible tasks for a project sequentially.
// It plans pending tasks and runs planned tasks, blocking between each.
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

		task := tasks[0]
		if cfg.Debug {
			log.Printf("[debug][autopilot] selected task #%d (%s) state=%s from %d candidates", task.ID, task.Description, queue.EffectiveState(task), len(tasks))
		}
		state := queue.EffectiveState(task)

		if reason := CheckGuardrails(cfg); reason != "" {
			emit(logs.LevelWarn, fmt.Sprintf("Guardrail stopped %s: %s", project, reason))
			return
		}

		skeleton := config.SkeletonFor(cfg, project)

		// Plan if pending
		if state == queue.StatePending {
			emit(logs.LevelInfo, fmt.Sprintf("Planning task #%d: %s", task.ID, task.Description))
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePending, Message: "planning"})
			p, planErr := planFn(task, skeleton)
			if planErr != nil {
				emit(logs.LevelError, fmt.Sprintf("Plan failed for task #%d: %v", task.ID, planErr))
				queue.SetFailReason(task.ID, "plan generation failed: "+planErr.Error())
				if !task.Optional {
					emit(logs.LevelError, fmt.Sprintf("Chain stopped for %s: task #%d failed", project, task.ID))
					return
				}
				emit(logs.LevelWarn, fmt.Sprintf("Optional task #%d failed, continuing chain", task.ID))
				continue
			}
			// Notify UI that plan is ready (PLN state)
			emit(logs.LevelSuccess, fmt.Sprintf("Plan ready for task #%d", task.ID))
			send(TaskStateMsg{TaskID: task.ID, State: queue.StatePlanned})
			// Brief pause so UI can reflect PLN state before transitioning to RUN
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			runOneTask(ctx, task, p, cfg, send, mgr, emit)
		} else if state == queue.StatePlanned {
			// Already planned, parse and run
			p, parseErr := plan.ParsePlan(plan.PlanPath(task.ID))
			if parseErr != nil {
				emit(logs.LevelError, fmt.Sprintf("Plan parse failed for task #%d: %v", task.ID, parseErr))
				queue.SetFailReason(task.ID, "plan parse failed: "+parseErr.Error())
				if !task.Optional {
					emit(logs.LevelError, fmt.Sprintf("Chain stopped for %s: task #%d failed", project, task.ID))
					return
				}
				emit(logs.LevelWarn, fmt.Sprintf("Optional task #%d failed, continuing chain", task.ID))
				continue
			}
			runOneTask(ctx, task, p, cfg, send, mgr, emit)
		}

		// Check if the task failed after execution — stop chain for non-optional
		updated, _ := queue.GetTask(task.ID)
		if queue.EffectiveState(updated) == queue.StateFailed {
			if !task.Optional {
				emit(logs.LevelError, fmt.Sprintf("Chain stopped for %s: task #%d failed", project, task.ID))
				return
			}
			emit(logs.LevelWarn, fmt.Sprintf("Optional task #%d failed, continuing chain", task.ID))
		}

		// Small delay between tasks
		select {
		case <-ctx.Done():
			emit(logs.LevelWarn, fmt.Sprintf("Project autopilot stopped for %s", project))
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// runOneTask starts a single task via the engine manager and waits for completion or cancellation.
func runOneTask(ctx context.Context, task queue.Task, p plan.Plan, cfg config.Config, send func(tea.Msg), mgr *Manager, emit func(logs.LogLevel, string)) {
	taskDone := make(chan struct{})

	// Wrap send to detect task completion
	wrappedSend := func(msg tea.Msg) {
		send(msg)
		if tsm, ok := msg.(TaskStateMsg); ok {
			if tsm.TaskID == task.ID && (tsm.State == queue.StateDone || tsm.State == queue.StateFailed) {
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
		mgr.Stop(task.ID)
		return
	case <-taskDone:
		// Task finished (done or failed), continue loop
	}
}
