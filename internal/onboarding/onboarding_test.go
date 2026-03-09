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

func TestOptionalMCPServers_PencilExists(t *testing.T) {
	found := false
	for _, m := range optionalMCPServers {
		if m.name == "pencil" {
			found = true
			if m.command != "pencil-mcp" {
				t.Fatalf("expected pencil command 'pencil-mcp', got %q", m.command)
			}
			if m.setupFunc == nil {
				t.Fatal("pencil setupFunc should not be nil")
			}
		}
	}
	if !found {
		t.Fatal("pencil not found in optionalMCPServers")
	}
}

func TestListOptionalMCP(t *testing.T) {
	items := ListOptionalMCP()
	if len(items) == 0 {
		t.Fatal("expected at least one optional MCP")
	}
	found := false
	for _, item := range items {
		if item.Name == "pencil" {
			found = true
			if item.Description == "" {
				t.Fatal("pencil description should not be empty")
			}
		}
	}
	if !found {
		t.Fatal("pencil not found in ListOptionalMCP result")
	}
}

func TestInstallPencilWrapper(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := installPencilWrapper()
	if err != nil {
		t.Fatalf("installPencilWrapper failed: %v", err)
	}

	wrapperPath := filepath.Join(tmpDir, ".local", "bin", "pencil-mcp")
	data, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("wrapper not created: %v", err)
	}
	content := string(data)
	if len(content) < 50 {
		t.Fatal("wrapper script too short")
	}
	// Verify it contains the find command for Pencil
	if !contains(content, "mcp-server-linux-x64") {
		t.Fatal("wrapper missing mcp-server-linux-x64 reference")
	}
	// Verify executable
	info, _ := os.Stat(wrapperPath)
	if info.Mode()&0111 == 0 {
		t.Fatal("wrapper is not executable")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestMergeHooksIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Run twice — globalPreToolUse returns nil, so no hooks are added globally.
	// Bash hooks live per-project via InstallHooks(), not in global settings.
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

	// globalPreToolUse returns nil — no matchers added to global settings
	if len(hb.PreToolUse) != 0 {
		t.Fatalf("expected 0 matchers (hooks are per-project), got %d", len(hb.PreToolUse))
	}
}
