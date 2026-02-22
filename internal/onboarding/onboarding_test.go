package onboarding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeMatcher_NewMatcher(t *testing.T) {
	existing := []hookMatcher{}
	desired := hookMatcher{
		Matcher: "Bash",
		Hooks:   []hookEntry{{Type: "command", Command: "/test/hook.sh", Timeout: 5000}},
	}
	result := mergeMatcher(existing, desired)
	if len(result) != 1 {
		t.Fatalf("expected 1 matcher, got %d", len(result))
	}
	if result[0].Matcher != "Bash" {
		t.Fatalf("expected Bash matcher, got %s", result[0].Matcher)
	}
	if len(result[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(result[0].Hooks))
	}
}

func TestMergeMatcher_NoDuplicates(t *testing.T) {
	existing := []hookMatcher{
		{
			Matcher: "Bash",
			Hooks:   []hookEntry{{Type: "command", Command: "/test/hook.sh", Timeout: 5000}},
		},
	}
	desired := hookMatcher{
		Matcher: "Bash",
		Hooks:   []hookEntry{{Type: "command", Command: "/test/hook.sh", Timeout: 5000}},
	}
	result := mergeMatcher(existing, desired)
	if len(result[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook (no duplicate), got %d", len(result[0].Hooks))
	}
}

func TestMergeMatcher_AddsNew(t *testing.T) {
	existing := []hookMatcher{
		{
			Matcher: "Bash",
			Hooks:   []hookEntry{{Type: "command", Command: "/test/old.sh", Timeout: 5000}},
		},
	}
	desired := hookMatcher{
		Matcher: "Bash",
		Hooks:   []hookEntry{{Type: "command", Command: "/test/new.sh", Timeout: 5000}},
	}
	result := mergeMatcher(existing, desired)
	if len(result[0].Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(result[0].Hooks))
	}
}

func TestMergeHooksIntoSettings_PreservesOtherKeys(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	initial := map[string]interface{}{
		"mcpServers":     map[string]interface{}{"context7": map[string]interface{}{"command": "npx"}},
		"enabledPlugins": map[string]interface{}{"test-plugin": true},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(settingsPath, data, 0644)

	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)

	err := mergeHooksIntoSettings(settingsPath, hooksDir)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Read back and verify other keys preserved
	result, _ := os.ReadFile(settingsPath)
	var raw map[string]json.RawMessage
	json.Unmarshal(result, &raw)

	if _, ok := raw["mcpServers"]; !ok {
		t.Fatal("mcpServers key was lost")
	}
	if _, ok := raw["enabledPlugins"]; !ok {
		t.Fatal("enabledPlugins key was lost")
	}
	if _, ok := raw["hooks"]; !ok {
		t.Fatal("hooks key was not added")
	}
}

func TestMergeHooksIntoSettings_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)

	err := mergeHooksIntoSettings(settingsPath, hooksDir)
	if err != nil {
		t.Fatalf("merge failed on new file: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	if _, ok := raw["hooks"]; !ok {
		t.Fatal("hooks key missing in new file")
	}
}

func TestMergeHooksIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Run twice
	mergeHooksIntoSettings(settingsPath, hooksDir)
	mergeHooksIntoSettings(settingsPath, hooksDir)

	data, _ := os.ReadFile(settingsPath)
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	type hooksBlock struct {
		PreToolUse []hookMatcher `json:"PreToolUse"`
	}
	var hb hooksBlock
	json.Unmarshal(raw["hooks"], &hb)

	// Should have exactly 2 matchers (Bash and Write|Edit), not 4
	if len(hb.PreToolUse) != 2 {
		t.Fatalf("expected 2 matchers after idempotent merge, got %d", len(hb.PreToolUse))
	}

	// Bash should have exactly 4 hooks, not 8
	for _, m := range hb.PreToolUse {
		if m.Matcher == "Bash" && len(m.Hooks) != 4 {
			t.Fatalf("expected 4 Bash hooks, got %d", len(m.Hooks))
		}
	}
}
