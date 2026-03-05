package backend

import (
	"strings"
	"testing"

	"github.com/JuanVilla424/teamoon/internal/config"
)

func TestClaudeBackend_Name(t *testing.T) {
	b := &ClaudeBackend{}
	if b.Name() != "claude" {
		t.Errorf("expected claude, got %s", b.Name())
	}
}

func TestClaudeBackend_Caps(t *testing.T) {
	b := &ClaudeBackend{}
	caps := b.Caps()
	if !caps.JSONStreaming {
		t.Error("JSONStreaming should be true")
	}
	if !caps.SessionResume {
		t.Error("SessionResume should be true")
	}
	if !caps.MCPConfig {
		t.Error("MCPConfig should be true")
	}
	if !caps.ToolFiltering {
		t.Error("ToolFiltering should be true")
	}
}

func TestClaudeBackend_ResolveModel_OpusPlan(t *testing.T) {
	b := &ClaudeBackend{}
	if m := b.ResolveModel("opusplan", "plan"); m != "claude-opus-4-6" {
		t.Errorf("opusplan+plan should be opus, got %s", m)
	}
	if m := b.ResolveModel("opusplan", "chat"); m != "claude-opus-4-6" {
		t.Errorf("opusplan+chat should be opus, got %s", m)
	}
	if m := b.ResolveModel("opusplan", "exec"); m != "claude-sonnet-4-6" {
		t.Errorf("opusplan+exec should be sonnet, got %s", m)
	}
	if m := b.ResolveModel("opusplan", "job"); m != "claude-sonnet-4-6" {
		t.Errorf("opusplan+job should be sonnet, got %s", m)
	}
}

func TestClaudeBackend_ResolveModel_Named(t *testing.T) {
	b := &ClaudeBackend{}
	tests := map[string]string{
		"opus":   "claude-opus-4-6",
		"sonnet": "claude-sonnet-4-6",
		"haiku":  "claude-haiku-4-5-20251001",
	}
	for input, expected := range tests {
		if m := b.ResolveModel(input, "exec"); m != expected {
			t.Errorf("ResolveModel(%s) = %s, want %s", input, m, expected)
		}
	}
}

func TestClaudeBackend_ResolveModel_Passthrough(t *testing.T) {
	b := &ClaudeBackend{}
	if m := b.ResolveModel("claude-opus-4-6", "exec"); m != "claude-opus-4-6" {
		t.Errorf("full model ID should pass through, got %s", m)
	}
}

func TestClaudeBackend_BuildArgs_Default(t *testing.T) {
	b := &ClaudeBackend{}
	args, cleanup := b.BuildArgs(SpawnRequest{
		Prompt:   "test prompt",
		MaxTurns: 15,
	})
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArg(args, "-p") {
		t.Error("missing -p flag")
	}
	if !containsArg(args, "--output-format") {
		t.Error("missing --output-format flag")
	}
	if !containsArgValue(args, "--max-turns", "15") {
		t.Error("max-turns should be 15")
	}
	if !containsArg(args, "--no-session-persistence") {
		t.Error("should have --no-session-persistence without session")
	}
}

func TestClaudeBackend_BuildArgs_WithSession(t *testing.T) {
	b := &ClaudeBackend{}
	args, cleanup := b.BuildArgs(SpawnRequest{
		Prompt:    "test",
		SessionID: "session-abc",
		MaxTurns:  10,
	})
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArgValue(args, "--resume", "session-abc") {
		t.Error("--resume should be present")
	}
	if containsArg(args, "--no-session-persistence") {
		t.Error("--no-session-persistence should NOT be present when resuming")
	}
}

func TestClaudeBackend_BuildArgs_ZeroMaxTurns(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{Prompt: "test", MaxTurns: 0})
	if containsArg(args, "--max-turns") {
		t.Error("MaxTurns=0 should omit --max-turns")
	}
}

func TestClaudeBackend_BuildArgs_NegativeMaxTurns(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{Prompt: "test", MaxTurns: -1})
	if !containsArgValue(args, "--max-turns", "15") {
		t.Error("negative MaxTurns should fall back to 15")
	}
}

func TestClaudeBackend_BuildArgs_WithModel(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{Prompt: "test", Model: "sonnet"})
	if !containsArgValue(args, "--model", "sonnet") {
		t.Error("--model should be present")
	}
}

func TestClaudeBackend_BuildArgs_WithEffort(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{Prompt: "test", Effort: "high"})
	if !containsArgValue(args, "--effort", "high") {
		t.Error("--effort should be present")
	}
}

func TestClaudeBackend_BuildArgs_WithAddDirs(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{
		Prompt:  "test",
		AddDirs: []string{"/tmp/a", "/tmp/b"},
	})
	count := 0
	for _, a := range args {
		if a == "--add-dir" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 --add-dir flags, got %d", count)
	}
}

func TestClaudeBackend_BuildArgs_WithMCPConfig(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{Prompt: "test", MCPConfig: "/tmp/mcp.json"})
	if !containsArgValue(args, "--mcp-config", "/tmp/mcp.json") {
		t.Error("--mcp-config should be present")
	}
}

func TestClaudeBackend_BuildArgs_WithDisallowedTools(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{
		Prompt:          "test",
		DisallowedTools: []string{"Bash", "Write"},
	})
	if !containsArg(args, "--disallowedTools") {
		t.Error("--disallowedTools should be present")
	}
}

func TestClaudeBackend_BuildArgs_WithPermissionMode(t *testing.T) {
	b := &ClaudeBackend{}
	args, _ := b.BuildArgs(SpawnRequest{
		Prompt:         "test",
		PermissionMode: "plan",
	})
	if !containsArg(args, "--permission-mode") {
		t.Error("--permission-mode should be present")
	}
	for i, a := range args {
		if a == "--permission-mode" && i+1 < len(args) {
			if args[i+1] != "plan" {
				t.Errorf("expected plan, got %s", args[i+1])
			}
			return
		}
	}
	t.Error("--permission-mode plan not found")
}

func TestFilterEnv_RemovesTarget(t *testing.T) {
	env := []string{"PATH=/usr/bin", "CLAUDECODE=abc", "HOME=/home/user"}
	filtered := FilterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(filtered))
	}
	for _, e := range filtered {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Error("CLAUDECODE should be removed")
		}
	}
}

func TestFilterEnv_KeepsOthers(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/home/user"}
	filtered := FilterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries, got %d", len(filtered))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	filtered := FilterEnv(nil, "FOO")
	if len(filtered) != 0 {
		t.Error("empty input should produce empty output")
	}
}

func TestFormatToolCall_Known(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected string
	}{
		{"Read", map[string]any{"file_path": "/foo.go"}, "Read /foo.go"},
		{"Write", map[string]any{"file_path": "/bar.go"}, "Write /bar.go"},
		{"Edit", map[string]any{"file_path": "/baz.go"}, "Edit /baz.go"},
		{"Glob", map[string]any{"pattern": "*.go"}, "Glob *.go"},
		{"Grep", map[string]any{"pattern": "func main"}, "Grep func main"},
		{"Bash", map[string]any{"command": "ls -la"}, "Bash: ls -la"},
		{"WebSearch", map[string]any{"query": "golang"}, "WebSearch: golang"},
	}
	for _, tt := range tests {
		result := FormatToolCall(tt.name, tt.input)
		if result != tt.expected {
			t.Errorf("FormatToolCall(%s) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}

func TestFormatToolCall_MCP(t *testing.T) {
	result := FormatToolCall("mcp__context7__query-docs", nil)
	if result != "context7: query docs" {
		t.Errorf("MCP format = %q, want %q", result, "context7: query docs")
	}
}

func TestFormatToolCall_Unknown(t *testing.T) {
	result := FormatToolCall("CustomTool", nil)
	if result != "CustomTool" {
		t.Errorf("unknown tool should return name, got %q", result)
	}
}

func TestExtractResult_Found(t *testing.T) {
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}
{"type":"result","result":"done successfully"}`
	result := ExtractResult(raw)
	if result != "done successfully" {
		t.Errorf("expected result text, got %q", result)
	}
}

func TestExtractResult_NotFound(t *testing.T) {
	raw := `{"type":"assistant","message":{"content":[]}}`
	result := ExtractResult(raw)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractResult_Truncated(t *testing.T) {
	long := strings.Repeat("x", 500)
	raw := `{"type":"result","result":"` + long + `"}`
	result := ExtractResult(raw)
	if len(result) > 310 {
		t.Errorf("result should be truncated, got len %d", len(result))
	}
}

func TestBuildMCPArgs_NilServers(t *testing.T) {
	cfg := config.Config{}
	mcpConfig, tools, cleanup := BuildMCPArgs(cfg)
	if mcpConfig != "" || tools != nil || cleanup != nil {
		t.Error("nil MCPServers should return empty")
	}
}

func TestBuildMCPArgs_DisabledServersSkipped(t *testing.T) {
	cfg := config.Config{
		MCPServers: map[string]config.MCPServer{
			"context7": {Command: "npx", Enabled: false},
		},
	}
	mcpConfig, tools, cleanup := BuildMCPArgs(cfg)
	if mcpConfig != "" || tools != nil || cleanup != nil {
		t.Error("disabled servers should be skipped")
	}
}

// helpers

func containsArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func containsArgValue(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}
