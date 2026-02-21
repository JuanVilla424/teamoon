package metrics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type Message struct {
	Model string `json:"model"`
	Usage Usage  `json:"usage"`
}

type jsonlEntry struct {
	Message   *Message `json:"message,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
}

type TokenSummary struct {
	Input        int    `json:"input"`
	Output       int    `json:"output"`
	CacheRead    int    `json:"cache_read"`
	CacheCreate  int    `json:"cache_create"`
	Total        int    `json:"total"`
	LastModel    string `json:"last_model"`
	SessionCount int    `json:"session_count"`
}

type SessionContext struct {
	ContextTokens  int     `json:"context_tokens"`
	ContextLimit   int     `json:"context_limit"`
	ContextPercent float64 `json:"context_percent"`
	OutputTokens   int     `json:"output_tokens"`
	SessionFile    string  `json:"session_file"`
}

func ScanTokens(claudeDir string) (today TokenSummary, week TokenSummary, month TokenSummary, err error) {
	projectsDir := filepath.Join(claudeDir, "projects")
	dirs, err := os.ReadDir(projectsDir)
	if err != nil {
		return today, week, month, err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var latestModTime time.Time
	var latestModel string

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		sessionDir := filepath.Join(projectsDir, d.Name())
		files, err := filepath.Glob(filepath.Join(sessionDir, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}

			modTime := info.ModTime()
			if modTime.Before(monthStart) {
				continue
			}

			var fileModel string
			entries := parseJSONL(f)
			for _, e := range entries {
				if e.Message == nil || e.Message.Usage.InputTokens == 0 {
					continue
				}

				u := e.Message.Usage

				month.Input += u.InputTokens
				month.Output += u.OutputTokens
				month.CacheRead += u.CacheReadInputTokens
				month.CacheCreate += u.CacheCreationInputTokens
				if e.Message.Model != "" {
					fileModel = e.Message.Model
				}

				if modTime.After(weekStart) || modTime.Equal(weekStart) {
					week.Input += u.InputTokens
					week.Output += u.OutputTokens
					week.CacheRead += u.CacheReadInputTokens
					week.CacheCreate += u.CacheCreationInputTokens
				}

				if modTime.After(todayStart) || modTime.Equal(todayStart) {
					today.Input += u.InputTokens
					today.Output += u.OutputTokens
					today.CacheRead += u.CacheReadInputTokens
					today.CacheCreate += u.CacheCreationInputTokens
				}
			}

			if fileModel != "" && modTime.After(latestModTime) {
				latestModTime = modTime
				latestModel = fileModel
			}

			if modTime.After(monthStart) {
				month.SessionCount++
			}
			if modTime.After(weekStart) {
				week.SessionCount++
			}
			if modTime.After(todayStart) {
				today.SessionCount++
			}
		}
	}

	month.LastModel = latestModel
	week.LastModel = latestModel
	today.LastModel = latestModel

	today.Total = today.Input + today.Output + today.CacheRead
	week.Total = week.Input + week.Output + week.CacheRead
	month.Total = month.Input + month.Output + month.CacheRead

	return today, week, month, nil
}

func ScanActiveSession(claudeDir string, contextLimit int) SessionContext {
	projectsDir := filepath.Join(claudeDir, "projects")
	dirs, _ := os.ReadDir(projectsDir)

	var newestFile string
	var newestTime time.Time

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		sessionDir := filepath.Join(projectsDir, d.Name())
		files, _ := filepath.Glob(filepath.Join(sessionDir, "*.jsonl"))
		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.ModTime().After(newestTime) {
				newestTime = info.ModTime()
				newestFile = f
			}
		}
	}

	if newestFile == "" {
		return SessionContext{ContextLimit: contextLimit}
	}

	entries := parseJSONL(newestFile)

	var lastContext int
	var totalOutput int
	for _, e := range entries {
		if e.Message == nil {
			continue
		}
		u := e.Message.Usage
		ctx := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		if ctx > 0 {
			lastContext = ctx
		}
		totalOutput += u.OutputTokens
	}

	pct := 0.0
	if contextLimit > 0 && lastContext > 0 {
		pct = float64(lastContext) / float64(contextLimit) * 100
	}

	return SessionContext{
		ContextTokens:  lastContext,
		ContextLimit:   contextLimit,
		ContextPercent: pct,
		OutputTokens:   totalOutput,
		SessionFile:    filepath.Base(newestFile),
	}
}

func parseJSONL(path string) []jsonlEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []jsonlEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry jsonlEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}
