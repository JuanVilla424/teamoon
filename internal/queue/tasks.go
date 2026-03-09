package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type TaskState string

const (
	StatePending  TaskState = "pending"
	StatePlanned  TaskState = "planned"
	StateRunning  TaskState = "running"
	StateDone     TaskState = "done"
	StateArchived TaskState = "archived"
)

type Task struct {
	ID          int       `json:"id"`
	Project     string    `json:"project"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	State       TaskState `json:"state,omitempty"`
	PlanFile    string    `json:"plan_file,omitempty"`
	FailReason  string    `json:"fail_reason,omitempty"`
	Done        bool      `json:"done"`
	AutoPilot   bool      `json:"auto_pilot"`
	Optional    bool      `json:"optional,omitempty"`
	Assignee     string   `json:"assignee,omitempty"`
	Attachments  []string `json:"attachments,omitempty"`
	PlanAttempts int      `json:"plan_attempts,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	CurrentStep  int      `json:"current_step,omitempty"`
	TotalSteps   int      `json:"total_steps,omitempty"`
	Wave         int      `json:"wave,omitempty"`
}

func EffectiveState(t Task) TaskState {
	if t.State != "" {
		return t.State
	}
	if t.Done {
		return StateDone
	}
	return StatePending
}

type TaskStore struct {
	NextID int    `json:"next_id"`
	Tasks  []Task `json:"tasks"`
}

var (
	storeMu sync.Mutex
	cached  *TaskStore
	dirty   bool
)

func tasksPath() string {
	return filepath.Join(config.ConfigDir(), "tasks.json")
}

// InitStore loads the task store into memory at startup.
func InitStore() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	_, err := loadStore()
	return err
}

// FlushIfDirty writes the in-memory store to disk if it has been modified.
func FlushIfDirty() {
	storeMu.Lock()
	defer storeMu.Unlock()
	if !dirty || cached == nil {
		return
	}
	dir := config.ConfigDir()
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(tasksPath(), data, 0644); err != nil {
		log.Printf("[queue] flush error: %v", err)
		return
	}
	dirty = false
}

func loadStore() (TaskStore, error) {
	if cached != nil {
		return *cached, nil
	}
	store := TaskStore{NextID: 1}
	data, err := os.ReadFile(tasksPath())
	if err != nil {
		if os.IsNotExist(err) {
			cached = &store
			return store, nil
		}
		return store, err
	}
	// Migrate legacy states from disk
	data = bytes.ReplaceAll(data, []byte(`"state": "blocked"`), []byte(`"state": "pending"`))
	data = bytes.ReplaceAll(data, []byte(`"state":"blocked"`), []byte(`"state":"pending"`))
	data = bytes.ReplaceAll(data, []byte(`"state": "failed"`), []byte(`"state": "pending"`))
	data = bytes.ReplaceAll(data, []byte(`"state":"failed"`), []byte(`"state":"pending"`))
	data = bytes.ReplaceAll(data, []byte(`"block_reason"`), []byte(`"fail_reason"`))
	err = json.Unmarshal(data, &store)
	if err == nil {
		cached = &store
	}
	return store, err
}

func saveStore(store TaskStore) error {
	s := store
	cached = &s
	dirty = true
	return nil
}

func Add(project, description, priority string) (Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return Task{}, err
	}

	if priority == "" {
		priority = "med"
	}

	task := Task{
		ID:          store.NextID,
		Project:     project,
		Description: description,
		Priority:    priority,
		CreatedAt:   time.Now(),
		State:       StatePending,
	}
	store.NextID++
	store.Tasks = append(store.Tasks, task)

	if err := saveStore(store); err != nil {
		return Task{}, err
	}
	log.Printf("[queue] task #%d created: project=%s desc=%q", task.ID, task.Project, task.Description)
	notifyWebhook("task_created", task)
	return task, nil
}

func MarkDone(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}

	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].Done = true
			store.Tasks[i].State = StateDone
			store.Tasks[i].SessionID = ""
			store.Tasks[i].CurrentStep = 0
			if err := saveStore(store); err != nil {
				return err
			}
			log.Printf("[queue] task #%d marked done", id)
			notifyWebhook("task_done", store.Tasks[i])
			return nil
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func ListPending() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var pending []Task
	for _, t := range store.Tasks {
		if EffectiveState(t) != StateDone {
			pending = append(pending, t)
		}
	}
	return pending, nil
}

func ListActive() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var active []Task
	for _, t := range store.Tasks {
		if EffectiveState(t) != StateArchived {
			active = append(active, t)
		}
	}
	return active, nil
}

func Archive(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].State = StateArchived
			store.Tasks[i].Done = true
			store.Tasks[i].SessionID = ""
			store.Tasks[i].CurrentStep = 0
			log.Printf("[queue] task #%d archived", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func BulkArchive(ids []int) (int, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return 0, err
	}
	idSet := make(map[int]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	count := 0
	for i := range store.Tasks {
		if idSet[store.Tasks[i].ID] {
			store.Tasks[i].State = StateArchived
			store.Tasks[i].Done = true
			store.Tasks[i].SessionID = ""
			store.Tasks[i].CurrentStep = 0
			log.Printf("[queue] task #%d archived (bulk)", store.Tasks[i].ID)
			count++
		}
	}
	if count > 0 {
		if err := saveStore(store); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func ListAll() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	return store.Tasks, nil
}

// stateRank defines the forward-only ordering of task states.
// UpdateState only allows transitions to equal or higher rank.
var stateRank = map[TaskState]int{
	StatePending:  0,
	StatePlanned:  1,
	StateRunning:  2,
	StateDone:     3,
	StateArchived: 4,
}

// UpdateState transitions a task to a new state.
// Forward-only: transitions to a lower-ranked state are silently refused.
// Use ForceUpdateState for explicit user actions or crash recovery.
func UpdateState(id int, state TaskState) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			current := EffectiveState(store.Tasks[i])
			if stateRank[state] < stateRank[current] {
				log.Printf("[queue] task #%d REFUSED backward state %s -> %s", id, current, state)
				return nil
			}
			store.Tasks[i].State = state
			if state == StateDone {
				store.Tasks[i].Done = true
			}
			log.Printf("[queue] task #%d state -> %s", id, state)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// ForceUpdateState transitions a task to any state, bypassing forward-only guard.
// Use ONLY for explicit user actions (Reset button, Stop button) or crash recovery.
func ForceUpdateState(id int, state TaskState) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			old := store.Tasks[i].State
			store.Tasks[i].State = state
			if state == StateDone {
				store.Tasks[i].Done = true
			}
			log.Printf("[queue] task #%d state FORCE %s -> %s", id, old, state)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetPlanFile(id int, path string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].PlanFile = path
			store.Tasks[i].State = StatePlanned
			log.Printf("[queue] task #%d plan set: %s", id, path)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func ResetPlan(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].State = StatePending
			store.Tasks[i].PlanFile = ""
			store.Tasks[i].FailReason = ""
			store.Tasks[i].Done = false
			store.Tasks[i].PlanAttempts = 0
			store.Tasks[i].SessionID = ""
			store.Tasks[i].CurrentStep = 0
			log.Printf("[queue] task #%d plan reset", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// IncrementPlanAttempts atomically increments the plan attempt counter and returns the new value.
func IncrementPlanAttempts(id int) (int, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return 0, err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].PlanAttempts++
			count := store.Tasks[i].PlanAttempts
			if err := saveStore(store); err != nil {
				return 0, err
			}
			log.Printf("[queue] task #%d plan_attempts=%d", id, count)
			return count, nil
		}
	}
	return 0, fmt.Errorf("task #%d not found", id)
}

// SetFailReason stores a failure reason WITHOUT changing the task state.
// The task stays in its current state (forward-only guarantee).
func SetFailReason(id int, reason string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].FailReason = reason
			if err := saveStore(store); err != nil {
				return err
			}
			log.Printf("[queue] task #%d fail_reason: %s", id, reason)
			notifyWebhook("task_failed", store.Tasks[i])
			return nil
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func ResetFailReason(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].FailReason = ""
			log.Printf("[queue] task #%d fail_reason cleared", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func ToggleAutoPilot(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].AutoPilot = !store.Tasks[i].AutoPilot
			log.Printf("[queue] task #%d autopilot=%v", id, store.Tasks[i].AutoPilot)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetAllAutoPilot(on bool) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if EffectiveState(store.Tasks[i]) != StateDone {
			store.Tasks[i].AutoPilot = on
		}
	}
	return saveStore(store)
}

func GetTask(id int) (Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return Task{}, err
	}
	for _, t := range store.Tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("task #%d not found", id)
}

func ListAutopilotPending(project string) ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var result []Task
	for _, t := range store.Tasks {
		if t.Project != project || !t.AutoPilot {
			continue
		}
		s := EffectiveState(t)
		if s == StatePending || s == StatePlanned {
			result = append(result, t)
		}
	}

	// Wave order: wave ascending (0 treated as max int = legacy sequential last),
	// then ID ascending within each wave.
	sort.Slice(result, func(i, j int) bool {
		wi, wj := result[i].Wave, result[j].Wave
		if wi == 0 {
			wi = 1<<31 - 1
		}
		if wj == 0 {
			wj = 1<<31 - 1
		}
		if wi != wj {
			return wi < wj
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// ListAutopilotSystemPending returns system-assignee tasks with autopilot enabled.
func ListAutopilotSystemPending() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var result []Task
	for _, t := range store.Tasks {
		if t.Assignee != "system" || !t.AutoPilot {
			continue
		}
		s := EffectiveState(t)
		if s == StatePending || s == StatePlanned {
			result = append(result, t)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func priorityRank(p string) int {
	switch p {
	case "high":
		return 2
	case "med":
		return 1
	default:
		return 0
	}
}

// RecoverRunning resets tasks stuck in "running" state after a service restart.
// Only resets tasks WITHOUT a SessionID (those can't be resumed).
// Tasks with SessionID are left in running state for RecoverAndResume to handle.
// NOTE: This is a crash-recovery function — it intentionally bypasses the
// forward-only guard since the service crashed and the running state is stale.
func RecoverRunning() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var recovered []Task
	for i := range store.Tasks {
		if store.Tasks[i].State == StateRunning && store.Tasks[i].SessionID == "" {
			if store.Tasks[i].PlanFile != "" {
				store.Tasks[i].State = StatePlanned
			} else {
				store.Tasks[i].State = StatePending
			}
			recovered = append(recovered, store.Tasks[i])
		}
	}

	if len(recovered) > 0 {
		if err := saveStore(store); err != nil {
			return nil, err
		}
	}
	return recovered, nil
}

// AutopilotProjects returns distinct projects with autopilot-eligible tasks.
func AutopilotProjects() ([]string, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var projects []string
	for _, t := range store.Tasks {
		if !t.AutoPilot || seen[t.Project] {
			continue
		}
		s := EffectiveState(t)
		if s == StatePending || s == StatePlanned {
			seen[t.Project] = true
			projects = append(projects, t.Project)
		}
	}
	return projects, nil
}

func UpdateDescription(id int, description string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].Description = description
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func UpdateWave(id int, wave int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].Wave = wave
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func UpdateAssignee(id int, assignee string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].Assignee = assignee
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func AttachToTask(id int, uploadID string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].Attachments = append(store.Tasks[i].Attachments, uploadID)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetSessionID(id int, sid string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].SessionID = sid
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetCurrentStep(id int, step int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].CurrentStep = step
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetTotalSteps(id int, total int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].TotalSteps = total
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

// ListResumable returns tasks in running state that have a SessionID (can be resumed after restart).
func ListResumable() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var result []Task
	for _, t := range store.Tasks {
		if t.State == StateRunning && t.SessionID != "" {
			result = append(result, t)
		}
	}
	return result, nil
}

func notifyWebhook(event string, task Task) {
	cfg, _ := config.Load()
	if cfg.WebhookURL == "" {
		return
	}
	go func() {
		payload, _ := json.Marshal(map[string]any{
			"event": event,
			"task":  task,
			"time":  time.Now(),
		})
		http.Post(cfg.WebhookURL, "application/json", bytes.NewReader(payload))
	}()
}
