package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UsagePeriod represents a single usage quota period.
type UsagePeriod struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// ClaudeUsage holds the usage data parsed from interactive `claude /usage`.
type ClaudeUsage struct {
	Session    UsagePeriod `json:"session"`
	WeekAll    UsagePeriod `json:"week_all"`
	WeekSonnet UsagePeriod `json:"week_sonnet"`
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b`)
var pctUsedRe = regexp.MustCompile(`(\d+)\s*%\s*used`)
var resetsRe = regexp.MustCompile(`(?i)[Rr]eset?s?\s+(.+?)(?:\s{2,}|\n|$)`)

const expectTemplate = `#!/usr/bin/expect -f
set timeout 30
set env(CLAUDECODE) ""
set env(TERM) "xterm-256color"
log_file -noappend %s

spawn %s

expect {
    "shortcuts" {}
    timeout { exit 1 }
}

sleep 3
send "/usage"
sleep 1
send "\t"
sleep 1
send "\r"
sleep 10
send "/exit\r"
sleep 2
expect eof
`

// FetchClaudeUsage spawns an interactive claude session via expect,
// executes /usage (Tab to accept autocomplete + Enter to submit), and parses the log.
// projectDir should be a trusted project directory to avoid the trust prompt.
func FetchClaudeUsage(projectDir string) (ClaudeUsage, error) {
	// Temp file for expect log output
	tmpLog, err := os.CreateTemp("", "teamoon_usage_log_*.txt")
	if err != nil {
		return ClaudeUsage{}, fmt.Errorf("create log temp: %w", err)
	}
	tmpLogPath := tmpLog.Name()
	tmpLog.Close()
	defer os.Remove(tmpLogPath)

	// Temp file for expect script
	tmpExp, err := os.CreateTemp("", "teamoon_usage_*.exp")
	if err != nil {
		return ClaudeUsage{}, fmt.Errorf("create exp temp: %w", err)
	}
	tmpExpPath := tmpExp.Name()
	claudePath := findClaude()
	fmt.Fprintf(tmpExp, expectTemplate, tmpLogPath, claudePath)
	tmpExp.Close()
	os.Chmod(tmpExpPath, 0755)
	defer os.Remove(tmpExpPath)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "expect", tmpExpPath)
	cmd.Env = filterEnvKey(os.Environ(), "CLAUDECODE")
	cmd.Dir = projectDir
	cmd.Stdout = nil
	cmd.Stderr = nil

	_ = cmd.Run()

	out, err := os.ReadFile(tmpLogPath)
	if err != nil || len(out) == 0 {
		return ClaudeUsage{}, fmt.Errorf("read expect log: no output captured")
	}

	clean := stripAnsi(string(out))
	return parseUsageOutput(clean)
}

func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func parseUsageOutput(raw string) (ClaudeUsage, error) {
	var usage ClaudeUsage

	// Find all "X% used" occurrences in order:
	// 1st = session, 2nd = week (all models), 3rd = week (Sonnet only)
	matches := pctUsedRe.FindAllStringSubmatch(raw, -1)
	if len(matches) >= 1 {
		usage.Session.Utilization, _ = strconv.ParseFloat(matches[0][1], 64)
	}
	if len(matches) >= 2 {
		usage.WeekAll.Utilization, _ = strconv.ParseFloat(matches[1][1], 64)
	}
	if len(matches) >= 3 {
		usage.WeekSonnet.Utilization, _ = strconv.ParseFloat(matches[2][1], 64)
	}

	// Best-effort reset times from TUI output
	resetMatches := resetsRe.FindAllStringSubmatch(raw, -1)
	if len(resetMatches) >= 1 {
		usage.Session.ResetsAt = strings.TrimSpace(resetMatches[0][1])
	}
	if len(resetMatches) >= 2 {
		usage.WeekAll.ResetsAt = strings.TrimSpace(resetMatches[1][1])
	}

	return usage, nil
}

func findClaude() string {
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.local/bin/claude",
		"/usr/local/bin/claude",
		"/usr/bin/claude",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "claude"
}

func filterEnvKey(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// --- Shared background fetcher ---

var (
	usageMu    sync.RWMutex
	usageCache ClaudeUsage
	usageOnce  sync.Once
)

// StartUsageFetcher starts a single background goroutine that fetches
// claude /usage every 2 minutes. Must be called once at startup.
// projectDir is a trusted project directory to avoid Claude's trust prompt.
func StartUsageFetcher(projectDir string) {
	usageOnce.Do(func() {
		go func() {
			for {
				u, err := FetchClaudeUsage(projectDir)
				usageMu.Lock()
				if err == nil {
					usageCache = u
					log.Printf("[usage] session=%.0f%% week_all=%.0f%% week_sonnet=%.0f%% session_resets=%q week_resets=%q",
						u.Session.Utilization, u.WeekAll.Utilization, u.WeekSonnet.Utilization,
						u.Session.ResetsAt, u.WeekAll.ResetsAt)
				} else {
					log.Printf("[usage] fetch failed: %v", err)
				}
				usageMu.Unlock()
				time.Sleep(2 * time.Minute)
			}
		}()
	})
}

// GetUsage returns the last successfully fetched usage data (non-blocking).
func GetUsage() ClaudeUsage {
	usageMu.RLock()
	defer usageMu.RUnlock()
	return usageCache
}
