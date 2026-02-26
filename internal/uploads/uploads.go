package uploads

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
)

const MaxUploadSize = 10 << 20 // 10 MB

type Attachment struct {
	ID         string    `json:"id"`
	OrigName   string    `json:"orig_name"`
	MIMEType   string    `json:"mime_type"`
	Size       int64     `json:"size"`
	StoredName string    `json:"stored_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type store struct {
	Attachments []Attachment `json:"attachments"`
}

var mu sync.Mutex

func uploadsDir() string {
	return filepath.Join(config.ConfigDir(), "uploads")
}

func storePath() string {
	return filepath.Join(config.ConfigDir(), "uploads.json")
}

func AbsPath(a Attachment) string {
	return filepath.Join(uploadsDir(), a.StoredName)
}

func loadStore() (store, error) {
	var s store
	data, err := os.ReadFile(storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

func saveStore(s store) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storePath(), data, 0644)
}

func genID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Save(src io.Reader, origName, mimeType string, size int64) (Attachment, error) {
	mu.Lock()
	defer mu.Unlock()

	dir := uploadsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Attachment{}, fmt.Errorf("create uploads dir: %w", err)
	}

	id := genID()
	ext := filepath.Ext(origName)
	storedName := id + ext

	dst, err := os.Create(filepath.Join(dir, storedName))
	if err != nil {
		return Attachment{}, fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		os.Remove(filepath.Join(dir, storedName))
		return Attachment{}, fmt.Errorf("write file: %w", err)
	}

	att := Attachment{
		ID:         id,
		OrigName:   origName,
		MIMEType:   mimeType,
		Size:       written,
		StoredName: storedName,
		CreatedAt:  time.Now(),
	}

	s, err := loadStore()
	if err != nil {
		s = store{}
	}
	s.Attachments = append(s.Attachments, att)
	if err := saveStore(s); err != nil {
		return Attachment{}, err
	}
	return att, nil
}

func GetByID(id string) (Attachment, error) {
	mu.Lock()
	defer mu.Unlock()

	s, err := loadStore()
	if err != nil {
		return Attachment{}, err
	}
	for _, a := range s.Attachments {
		if a.ID == id {
			return a, nil
		}
	}
	return Attachment{}, fmt.Errorf("attachment %s not found", id)
}

func ResolveIDs(ids []string) []Attachment {
	mu.Lock()
	defer mu.Unlock()

	s, err := loadStore()
	if err != nil {
		return nil
	}
	index := make(map[string]Attachment, len(s.Attachments))
	for _, a := range s.Attachments {
		index[a.ID] = a
	}
	var result []Attachment
	for _, id := range ids {
		if a, ok := index[id]; ok {
			result = append(result, a)
		}
	}
	return result
}

func DeleteByID(id string) error {
	mu.Lock()
	defer mu.Unlock()

	s, err := loadStore()
	if err != nil {
		return err
	}
	for i, a := range s.Attachments {
		if a.ID == id {
			os.Remove(filepath.Join(uploadsDir(), a.StoredName))
			s.Attachments = append(s.Attachments[:i], s.Attachments[i+1:]...)
			return saveStore(s)
		}
	}
	return fmt.Errorf("attachment %s not found", id)
}

func Cleanup(maxAge time.Duration) int {
	mu.Lock()
	defer mu.Unlock()

	s, err := loadStore()
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	var kept []Attachment
	removed := 0
	for _, a := range s.Attachments {
		if a.CreatedAt.Before(cutoff) {
			os.Remove(filepath.Join(uploadsDir(), a.StoredName))
			removed++
		} else {
			kept = append(kept, a)
		}
	}
	if removed > 0 {
		s.Attachments = kept
		saveStore(s)
	}
	return removed
}

func IsTextMIME(mime string) bool {
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	textTypes := []string{
		"application/json",
		"application/xml",
		"application/yaml",
		"application/x-yaml",
		"application/javascript",
		"application/typescript",
	}
	for _, t := range textTypes {
		if mime == t {
			return true
		}
	}
	return false
}
