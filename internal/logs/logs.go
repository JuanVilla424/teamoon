package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const logPath = "/var/log/teamoon.log"

func taskLogDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamoon", "logs")
}

func taskLogPath(taskID int) string {
	return filepath.Join(taskLogDir(), fmt.Sprintf("task-%d.log", taskID))
}

type LogLevel int

const (
	LevelDebug   LogLevel = -1
	LevelInfo    LogLevel = iota
	LevelSuccess
	LevelWarn
	LevelError
)

type LogEntry struct {
	Time    time.Time
	TaskID  int
	Project string
	Message string
	Level   LogLevel
	Agent   string
}

type RingBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	head    int
	size    int
	cap     int
	file    *os.File
	debug   bool
}

func (r *RingBuffer) SetDebug(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.debug = on
}

func NewRingBuffer(retentionDays int) *RingBuffer {
	if retentionDays <= 0 {
		retentionDays = 20
	}
	f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	rb := &RingBuffer{
		file: f,
	}
	rb.loadFromFileRetention(retentionDays)
	return rb
}

func (r *RingBuffer) loadFromFileRetention(retentionDays int) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		// No file yet — initialize with minimum capacity
		r.cap = 100
		r.entries = make([]LogEntry, r.cap)
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// Collect entries within retention window
	var kept []LogEntry
	for _, line := range lines {
		e := parseLogLine(line)
		if e.Time.IsZero() || e.Time.Before(cutoff) {
			continue
		}
		kept = append(kept, e)
	}
	cap := len(kept)
	if cap < 100 {
		cap = 100
	}
	r.entries = make([]LogEntry, cap)
	r.cap = cap
	for _, e := range kept {
		r.entries[r.head] = e
		r.head = (r.head + 1) % r.cap
		r.size++
	}
}

var levelTag = [...]string{"INFO", " OK ", "WARN", "ERR "}

func (r *RingBuffer) Add(e LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Skip debug entries unless debug mode is on
	if e.Level == LevelDebug && !r.debug {
		return
	}
	r.entries[r.head] = e
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
	tag := "INFO"
	if e.Level == LevelDebug {
		tag = "DBG "
	} else if int(e.Level) < len(levelTag) {
		tag = levelTag[e.Level]
	}
	msg := strings.ReplaceAll(e.Message, "\n", " ")
	var line string
	if e.Agent != "" {
		line = fmt.Sprintf("%s [%s] #%d %s [%s]: %s\n",
			e.Time.Format("2006-01-02 15:04:05"), tag, e.TaskID, e.Project, e.Agent, msg)
	} else {
		line = fmt.Sprintf("%s [%s] #%d %s: %s\n",
			e.Time.Format("2006-01-02 15:04:05"), tag, e.TaskID, e.Project, msg)
	}
	if r.file != nil {
		r.file.WriteString(line)
	}
	if e.TaskID > 0 {
		dir := taskLogDir()
		os.MkdirAll(dir, 0755)
		f, err := os.OpenFile(taskLogPath(e.TaskID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(line)
			f.Close()
		}
	}
}

func (r *RingBuffer) File() *os.File {
	return r.file
}

func (r *RingBuffer) Close() {
	if r.file != nil {
		r.file.Close()
	}
}

func ReadTaskLog(taskID int) []LogEntry {
	// Try per-task log file first (fast path)
	taskFile := taskLogPath(taskID)
	data, err := os.ReadFile(taskFile)
	if err != nil {
		// Fallback to global log scan for historical entries
		data, err = os.ReadFile(logPath)
		if err != nil {
			return nil
		}
	}

	needle := fmt.Sprintf("#%d ", taskID)
	var entries []LogEntry
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		e := parseLogLine(line)
		if e.TaskID == taskID {
			entries = append(entries, e)
		}
	}
	return entries
}

func parseLogLine(line string) LogEntry {
	// Format: 2006-01-02 15:04:05 [TAG ] #ID project: message
	var e LogEntry
	if len(line) < 27 {
		return e
	}

	t, err := time.Parse("2006-01-02 15:04:05", line[:19])
	if err != nil {
		return e
	}
	e.Time = t

	tag := ""
	if line[20] == '[' {
		end := strings.Index(line[20:], "]")
		if end > 0 {
			tag = strings.TrimSpace(line[21 : 20+end])
		}
	}
	switch tag {
	case "OK":
		e.Level = LevelSuccess
	case "WARN":
		e.Level = LevelWarn
	case "ERR":
		e.Level = LevelError
	case "DBG":
		e.Level = LevelDebug
	default:
		e.Level = LevelInfo
	}

	// Find #ID
	rest := line[26:]
	hashIdx := strings.Index(rest, "#")
	if hashIdx < 0 {
		return e
	}
	rest = rest[hashIdx+1:]
	spIdx := strings.Index(rest, " ")
	if spIdx < 0 {
		return e
	}
	fmt.Sscanf(rest[:spIdx], "%d", &e.TaskID)

	// project [agent]: message  OR  project: message
	rest = rest[spIdx+1:]
	colonIdx := strings.Index(rest, ": ")
	if colonIdx >= 0 {
		projPart := rest[:colonIdx]
		// Check for [agent] suffix in project part
		if bracketOpen := strings.Index(projPart, " ["); bracketOpen >= 0 {
			if bracketClose := strings.Index(projPart[bracketOpen+2:], "]"); bracketClose >= 0 {
				e.Project = projPart[:bracketOpen]
				e.Agent = projPart[bracketOpen+2 : bracketOpen+2+bracketClose]
			} else {
				e.Project = projPart
			}
		} else {
			e.Project = projPart
		}
		e.Message = rest[colonIdx+2:]
	} else {
		e.Message = rest
	}

	return e
}

// CleanupLogs removes log entries older than retentionDays.
// Rewrites the global log file and deletes old per-task log files.
func CleanupLogs(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Rewrite global log file keeping only recent entries
	data, err := os.ReadFile(logPath)
	if err == nil && len(data) > 0 {
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		var kept []string
		for _, line := range lines {
			e := parseLogLine(line)
			if !e.Time.IsZero() && !e.Time.Before(cutoff) {
				kept = append(kept, line)
			}
		}
		os.WriteFile(logPath, []byte(strings.Join(kept, "\n")+"\n"), 0644)
	}

	// Delete old per-task log files
	dir := taskLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "task-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func (r *RingBuffer) Snapshot() []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return nil
	}
	result := make([]LogEntry, r.size)
	start := (r.head - r.size + r.cap) % r.cap
	for i := 0; i < r.size; i++ {
		result[i] = r.entries[(start+i)%r.cap]
	}
	return result
}
