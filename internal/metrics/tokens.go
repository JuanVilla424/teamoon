package metrics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// contextLimitForModel returns the context window size for known Claude models.
func contextLimitForModel(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return 200000
	case strings.Contains(m, "sonnet"):
		return 200000
	case strings.Contains(m, "haiku"):
		return 200000
	default:
		return 200000 // safe default for Claude models
	}
}

// modelTier returns "opus", "sonnet", or "haiku" from a model string.
func modelTier(model string) string {
	m := strings.ToLower(model)
	if strings.Contains(m, "opus") {
		return "opus"
	}
	if strings.Contains(m, "haiku") {
		return "haiku"
	}
	return "sonnet" // default
}

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

// ModelTokens tracks tokens per model tier for accurate cost calculation.
type ModelTokens struct {
	Input       int
	Output      int
	CacheRead   int
	CacheCreate int
}

type TokenSummary struct {
	Input        int                     `json:"input"`
	Output       int                     `json:"output"`
	CacheRead    int                     `json:"cache_read"`
	CacheCreate  int                     `json:"cache_create"`
	Total        int                     `json:"total"`
	LastModel    string                  `json:"last_model"`
	SessionCount int                     `json:"session_count"`
	ByModel      map[string]*ModelTokens `json:"-"` // keyed by tier: "opus", "sonnet", "haiku"
}

func (s *TokenSummary) addUsage(u Usage, tier string) {
	s.Input += u.InputTokens
	s.Output += u.OutputTokens
	s.CacheRead += u.CacheReadInputTokens
	s.CacheCreate += u.CacheCreationInputTokens
	if s.ByModel == nil {
		s.ByModel = make(map[string]*ModelTokens)
	}
	mt, ok := s.ByModel[tier]
	if !ok {
		mt = &ModelTokens{}
		s.ByModel[tier] = mt
	}
	mt.Input += u.InputTokens
	mt.Output += u.OutputTokens
	mt.CacheRead += u.CacheReadInputTokens
	mt.CacheCreate += u.CacheCreationInputTokens
}

type SessionContext struct {
	ContextTokens  int     `json:"context_tokens"`
	ContextLimit   int     `json:"context_limit"`
	ContextPercent float64 `json:"context_percent"`
	OutputTokens   int     `json:"output_tokens"`
	SessionFile    string  `json:"session_file"`
}

// collectJSONL finds all *.jsonl files recursively under a directory.
func collectJSONL(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files
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
		// Recursively find all jsonl files (including subagents/)
		files := collectJSONL(sessionDir)

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
			var fileTier string
			entries := parseJSONL(f)
			for _, e := range entries {
				if e.Message == nil || e.Message.Usage.InputTokens == 0 {
					continue
				}

				u := e.Message.Usage
				if e.Message.Model != "" {
					fileModel = e.Message.Model
					fileTier = modelTier(e.Message.Model)
				}
				tier := fileTier
				if tier == "" {
					tier = "sonnet"
				}

				month.addUsage(u, tier)

				if modTime.After(weekStart) || modTime.Equal(weekStart) {
					week.addUsage(u, tier)
				}

				if modTime.After(todayStart) || modTime.Equal(todayStart) {
					today.addUsage(u, tier)
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
		// Include subagent files too
		files := collectJSONL(sessionDir)
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
	var lastModel string
	for _, e := range entries {
		if e.Message == nil {
			continue
		}
		if e.Message.Model != "" {
			lastModel = e.Message.Model
		}
		u := e.Message.Usage
		ctx := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		if ctx > 0 {
			lastContext = ctx
		}
		totalOutput += u.OutputTokens
	}

	// Auto-detect context limit from model if not configured
	limit := contextLimit
	if limit == 0 && lastModel != "" {
		limit = contextLimitForModel(lastModel)
	}

	pct := 0.0
	if limit > 0 && lastContext > 0 {
		pct = float64(lastContext) / float64(limit) * 100
		if pct > 100 {
			pct = 100
		}
	}

	return SessionContext{
		ContextTokens:  lastContext,
		ContextLimit:   limit,
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
