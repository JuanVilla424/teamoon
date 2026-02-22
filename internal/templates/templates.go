package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type Template struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type templateStore struct {
	NextID    int        `json:"next_id"`
	Templates []Template `json:"templates"`
}

var storeMu sync.Mutex

func templatesPath() string {
	return filepath.Join(config.ConfigDir(), "templates.json")
}

func loadStore() (templateStore, error) {
	store := templateStore{NextID: 1}
	data, err := os.ReadFile(templatesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	err = json.Unmarshal(data, &store)
	return store, err
}

func saveStore(store templateStore) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(templatesPath(), data, 0644)
}

func List() ([]Template, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	return store.Templates, nil
}

func Add(name, content string) (Template, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return Template{}, err
	}
	t := Template{
		ID:        store.NextID,
		Name:      name,
		Content:   content,
		CreatedAt: time.Now(),
	}
	store.NextID++
	store.Templates = append(store.Templates, t)
	return t, saveStore(store)
}

func Delete(id int) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	for i := range store.Templates {
		if store.Templates[i].ID == id {
			store.Templates = append(store.Templates[:i], store.Templates[i+1:]...)
			return saveStore(store)
		}
	}
	return fmt.Errorf("template #%d not found", id)
}

func Update(id int, name, content string) (Template, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return Template{}, err
	}
	for i := range store.Templates {
		if store.Templates[i].ID == id {
			store.Templates[i].Name = name
			store.Templates[i].Content = content
			return store.Templates[i], saveStore(store)
		}
	}
	return Template{}, fmt.Errorf("template #%d not found", id)
}
