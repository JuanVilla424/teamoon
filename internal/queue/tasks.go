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
	Assignee    string    `json:"assignee,omitempty"`
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

var storeMu sync.Mutex

func tasksPath() string {
	return filepath.Join(config.ConfigDir(), "tasks.json")
}

func loadStore() (TaskStore, error) {
	store := TaskStore{NextID: 1}
	data, err := os.ReadFile(tasksPath())
	if err != nil {
		if os.IsNotExist(err) {
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
	return store, err
}

func saveStore(store TaskStore) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tasksPath(), data, 0644)
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
			log.Printf("[queue] task #%d archived", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
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

func UpdateState(id int, state TaskState) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
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
			log.Printf("[queue] task #%d plan reset", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

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
			store.Tasks[i].State = StatePending
			if err := saveStore(store); err != nil {
				return err
			}
			log.Printf("[queue] task #%d back to pending: %s", id, reason)
			notifyWebhook("task_retry", store.Tasks[i])
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

	// FIFO order: process tasks in creation order (by ID).
	// Priority is informational only; it does not affect execution order.
	sort.Slice(result, func(i, j int) bool {
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
// Tasks with a plan file go back to "planned", others go back to "pending".
func RecoverRunning() ([]Task, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return nil, err
	}

	var recovered []Task
	for i := range store.Tasks {
		if store.Tasks[i].State == StateRunning {
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
