package jobs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type JobStatus string

const (
	StatusIdle    JobStatus = "idle"
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusError   JobStatus = "error"
)

type Job struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Schedule      string    `json:"schedule"`
	ScheduleHuman string    `json:"schedule_human"`
	Project       string    `json:"project"`
	Instruction   string    `json:"instruction"`
	Enabled       bool      `json:"enabled"`
	Status        JobStatus `json:"status"`
	LastRunAt     time.Time `json:"last_run_at,omitempty"`
	LastResult    string    `json:"last_result,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type jobStore struct {
	NextID int   `json:"next_id"`
	Jobs   []Job `json:"jobs"`
}

var storeMu sync.Mutex

func jobsPath() string {
	return filepath.Join(config.ConfigDir(), "jobs.json")
}

func loadStore() (jobStore, error) {
	store := jobStore{NextID: 1}
	data, err := os.ReadFile(jobsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	err = json.Unmarshal(data, &store)
	return store, err
}

func saveStore(store jobStore) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(jobsPath(), data, 0644)
}

func ListAll() ([]Job, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	if store.Jobs == nil {
		return []Job{}, nil
	}
	for i := range store.Jobs {
		store.Jobs[i].ScheduleHuman = HumanReadable(store.Jobs[i].Schedule)
	}
	return store.Jobs, nil
}

func GetByID(id int) (Job, bool) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, _ := loadStore()
	for _, j := range store.Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return Job{}, false
}

func Add(name, schedule, project, instruction string) (Job, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return Job{}, err
	}

	job := Job{
		ID:          store.NextID,
		Name:        name,
		Schedule:    schedule,
		Project:     project,
		Instruction: instruction,
		Enabled:     true,
		Status:      StatusIdle,
		CreatedAt:   time.Now(),
	}
	store.NextID++
	store.Jobs = append(store.Jobs, job)

	if err := saveStore(store); err != nil {
		return Job{}, err
	}
	log.Printf("[jobs] job #%d created: name=%q schedule=%q project=%s", job.ID, job.Name, job.Schedule, job.Project)
	return job, nil
}

func Update(id int, name, schedule, project, instruction string, enabled bool) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}

	for i := range store.Jobs {
		if store.Jobs[i].ID == id {
			store.Jobs[i].Name = name
			store.Jobs[i].Schedule = schedule
			store.Jobs[i].Project = project
			store.Jobs[i].Instruction = instruction
			store.Jobs[i].Enabled = enabled
			log.Printf("[jobs] job #%d updated", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("job #%d not found", id)
}

func Delete(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}

	for i := range store.Jobs {
		if store.Jobs[i].ID == id {
			store.Jobs = append(store.Jobs[:i], store.Jobs[i+1:]...)
			log.Printf("[jobs] job #%d deleted", id)
			return saveStore(store)
		}
	}
	return fmt.Errorf("job #%d not found", id)
}

func SetStatus(id int, status JobStatus) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return
	}
	for i := range store.Jobs {
		if store.Jobs[i].ID == id {
			store.Jobs[i].Status = status
			saveStore(store)
			return
		}
	}
}

func SetLastRun(id int, result string) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return
	}
	for i := range store.Jobs {
		if store.Jobs[i].ID == id {
			store.Jobs[i].LastRunAt = time.Now()
			store.Jobs[i].LastResult = result
			saveStore(store)
			return
		}
	}
}

func ToggleEnabled(id int) (bool, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return false, err
	}
	for i := range store.Jobs {
		if store.Jobs[i].ID == id {
			store.Jobs[i].Enabled = !store.Jobs[i].Enabled
			log.Printf("[jobs] job #%d enabled=%v", id, store.Jobs[i].Enabled)
			return store.Jobs[i].Enabled, saveStore(store)
		}
	}
	return false, fmt.Errorf("job #%d not found", id)
}
