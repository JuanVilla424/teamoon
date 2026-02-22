package onboarding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuanVilla424/teamoon/internal/projectinit"
)

func installGlobalHooks() error {
	fmt.Println("\n[5/7] Installing global hooks...")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	hooksDir := filepath.Join(home, ".claude", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	// Write each hook script to ~/.claude/hooks/
	files := projectinit.GlobalHookFiles()
	for name, content := range files {
		dest := filepath.Join(hooksDir, name)
		if err := os.WriteFile(dest, []byte(content), 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		fmt.Printf("  [+] %s\n", name)
	}

	// Merge PreToolUse entries into ~/.claude/settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := mergeHooksIntoSettings(settingsPath, hooksDir); err != nil {
		return fmt.Errorf("merging settings.json: %w", err)
	}

	fmt.Printf("  [+] settings.json hooks merged\n")
	return nil
}

// hookEntry represents a single hook entry in the PreToolUse array.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// hookMatcher represents one matcher+hooks pair in PreToolUse.
type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

// globalPreToolUse returns the desired PreToolUse configuration using absolute hook paths.
func globalPreToolUse(hooksDir string) []hookMatcher {
	return []hookMatcher{
		{
			Matcher: "Bash",
			Hooks: []hookEntry{
				{Type: "command", Command: filepath.Join(hooksDir, "security-check.sh"), Timeout: 5000},
				{Type: "command", Command: filepath.Join(hooksDir, "test-guard.sh"), Timeout: 5000},
				{Type: "command", Command: filepath.Join(hooksDir, "build-guard.sh"), Timeout: 5000},
				{Type: "command", Command: filepath.Join(hooksDir, "commit-format.sh"), Timeout: 5000},
			},
		},
		{
			Matcher: "Write|Edit",
			Hooks: []hookEntry{
				{Type: "command", Command: filepath.Join(hooksDir, "secrets-guard.sh"), Timeout: 5000},
			},
		},
	}
}

// mergeHooksIntoSettings reads settings.json, merges new PreToolUse entries that
// are not already present (by command path), then writes back.
func mergeHooksIntoSettings(settingsPath, hooksDir string) error {
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	}

	// Parse existing hooks block
	type hooksBlock struct {
		PreToolUse []hookMatcher `json:"PreToolUse"`
	}
	var existing hooksBlock
	if hooksRaw, ok := raw["hooks"]; ok {
		_ = json.Unmarshal(hooksRaw, &existing)
	}

	desired := globalPreToolUse(hooksDir)

	for _, desiredMatcher := range desired {
		existing.PreToolUse = mergeMatcher(existing.PreToolUse, desiredMatcher)
	}

	hooksJSON, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	raw["hooks"] = hooksJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0644)
}

// installHooksStream installs hooks to teamoon home with progress streaming.
func installHooksStream(progress ProgressFunc) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	tmHooksDir := filepath.Join(teamoonHome(), "hooks")
	if err := os.MkdirAll(tmHooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	// Write each hook script to teamoon home
	files := projectinit.GlobalHookFiles()
	for name, content := range files {
		dest := filepath.Join(tmHooksDir, name)
		if err := os.WriteFile(dest, []byte(content), 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		progress(map[string]any{"type": "hook", "name": name, "status": "done"})
	}

	// Create per-file symlinks in ~/.claude/hooks/
	claudeHooksDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(claudeHooksDir, 0755); err != nil {
		return fmt.Errorf("create claude hooks dir: %w", err)
	}
	for name := range files {
		target := filepath.Join(tmHooksDir, name)
		link := filepath.Join(claudeHooksDir, name)
		if err := ensureSymlink(target, link); err != nil {
			return fmt.Errorf("symlink %s: %w", name, err)
		}
	}
	progress(map[string]any{"type": "symlink", "name": "hooks", "status": "done"})

	// Merge hooks into settings.json — paths point to teamoon home
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := mergeHooksIntoSettings(settingsPath, tmHooksDir); err != nil {
		return fmt.Errorf("merge settings: %w", err)
	}
	progress(map[string]any{"type": "hook", "name": "settings.json", "status": "done"})

	return nil
}

// installGlobalHooksQuiet installs hooks without printing to stdout.
func installGlobalHooksQuiet() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	hooksDir := filepath.Join(home, ".claude", "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	files := projectinit.GlobalHookFiles()
	for name, content := range files {
		dest := filepath.Join(hooksDir, name)
		if err := os.WriteFile(dest, []byte(content), 0755); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	return mergeHooksIntoSettings(settingsPath, hooksDir)
}

// mergeMatcher inserts hooks from 'desired' into the matching entry in 'existing',
// creating the matcher entry if it does not exist. Deduplicates by command path.
func mergeMatcher(existing []hookMatcher, desired hookMatcher) []hookMatcher {
	for i, m := range existing {
		if m.Matcher == desired.Matcher {
			knownCommands := make(map[string]bool)
			for _, h := range m.Hooks {
				knownCommands[h.Command] = true
			}
			for _, h := range desired.Hooks {
				if !knownCommands[h.Command] {
					existing[i].Hooks = append(existing[i].Hooks, h)
				}
			}
			return existing
		}
	}
	return append(existing, desired)
}
