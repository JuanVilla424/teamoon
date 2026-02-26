package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

const maxMessages = 50

type Message struct {
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Project     string    `json:"project"`
	Timestamp   time.Time `json:"timestamp"`
	Attachments []string  `json:"attachments,omitempty"`
}

type chatStore struct {
	Messages []Message `json:"messages"`
}

var storeMu sync.Mutex

func chatPath() string {
	return filepath.Join(config.ConfigDir(), "chat.json")
}

func load() (chatStore, error) {
	var store chatStore
	data, err := os.ReadFile(chatPath())
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	err = json.Unmarshal(data, &store)
	return store, err
}

func save(store chatStore) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(chatPath(), data, 0644)
}

func LoadHistory() ([]Message, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := load()
	if err != nil {
		return nil, err
	}
	return store.Messages, nil
}

func AppendMessage(msg Message) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := load()
	if err != nil {
		return err
	}
	store.Messages = append(store.Messages, msg)
	if len(store.Messages) > maxMessages {
		store.Messages = store.Messages[len(store.Messages)-maxMessages:]
	}
	return save(store)
}

func ClearHistory() error {
	storeMu.Lock()
	defer storeMu.Unlock()
	return save(chatStore{})
}

func RecentContext(n int) []Message {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, _ := load()
	msgs := store.Messages
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	return msgs
}

func LoadHistoryForProject(project string) ([]Message, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := load()
	if err != nil {
		return nil, err
	}
	var filtered []Message
	for _, m := range store.Messages {
		if m.Project == project {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

func ClearHistoryForProject(project string) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := load()
	if err != nil {
		return err
	}
	var kept []Message
	for _, m := range store.Messages {
		if m.Project != project {
			kept = append(kept, m)
		}
	}
	store.Messages = kept
	return save(store)
}

func RecentContextForProject(n int, project string) []Message {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, _ := load()
	var filtered []Message
	for _, m := range store.Messages {
		if m.Project == project {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) > n {
		filtered = filtered[len(filtered)-n:]
	}
	return filtered
}
