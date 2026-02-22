package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type TaskState string

const (
	StatePending TaskState = "pending"
	StatePlanned TaskState = "planned"
	StateRunning TaskState = "running"
	StateBlocked TaskState = "blocked"
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
	BlockReason string    `json:"block_reason,omitempty"`
	Done        bool      `json:"done"`
	AutoPilot   bool      `json:"auto_pilot"`
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
			store.Tasks[i].BlockReason = ""
			store.Tasks[i].Done = false
			return saveStore(store)
		}
	}
	return fmt.Errorf("task #%d not found", id)
}

func SetBlockReason(id int, reason string) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			store.Tasks[i].BlockReason = reason
			store.Tasks[i].State = StateBlocked
			if err := saveStore(store); err != nil {
				return err
			}
			notifyWebhook("task_blocked", store.Tasks[i])
			return nil
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
