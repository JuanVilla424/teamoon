package backend

import (
	"strings"
	"testing"
)

func TestOpenCodeBackend_Name(t *testing.T) {
	b := &OpenCodeBackend{}
	if b.Name() != "opencode" {
		t.Errorf("expected opencode, got %s", b.Name())
	}
}

func TestOpenCodeBackend_Caps(t *testing.T) {
	b := &OpenCodeBackend{}
	caps := b.Caps()
	if !caps.JSONStreaming {
		t.Error("JSONStreaming should be true")
	}
	if !caps.SessionResume {
		t.Error("SessionResume should be true")
	}
	if caps.MCPConfig {
		t.Error("MCPConfig should be false for opencode")
	}
	if !caps.ToolFiltering {
		t.Error("ToolFiltering should be true")
	}
}

func TestOpenCodeBackend_ResolveModel_Opusplan_Plan(t *testing.T) {
	b := &OpenCodeBackend{}
	got := b.ResolveModel("opusplan", "plan")
	if got != "anthropic/claude-opus-4-6" {
		t.Errorf("expected anthropic/claude-opus-4-6, got %s", got)
	}
}

func TestOpenCodeBackend_ResolveModel_Opusplan_Exec(t *testing.T) {
	b := &OpenCodeBackend{}
	got := b.ResolveModel("opusplan", "exec")
	if got != "anthropic/claude-sonnet-4-6" {
		t.Errorf("expected anthropic/claude-sonnet-4-6, got %s", got)
	}
}

func TestOpenCodeBackend_ResolveModel_Named(t *testing.T) {
	b := &OpenCodeBackend{}
	tests := []struct {
		input, expected string
	}{
		{"opus", "anthropic/claude-opus-4-6"},
		{"sonnet", "anthropic/claude-sonnet-4-6"},
		{"haiku", "anthropic/claude-haiku-4-5-20251001"},
	}
	for _, tt := range tests {
		got := b.ResolveModel(tt.input, "exec")
		if got != tt.expected {
			t.Errorf("ResolveModel(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestOpenCodeBackend_ResolveModel_Passthrough(t *testing.T) {
	b := &OpenCodeBackend{}
	got := b.ResolveModel("openai/gpt-4o", "exec")
	if got != "openai/gpt-4o" {
		t.Errorf("expected passthrough, got %s", got)
	}
}

func TestOpenCodeBackend_BuildArgs_Default(t *testing.T) {
	b := &OpenCodeBackend{}
	args := b.BuildArgs(SpawnRequest{Prompt: "hello"})
	if args[0] != "run" {
		t.Error("first arg should be 'run'")
	}
	if args[1] != "hello" {
		t.Error("second arg should be the prompt")
	}
	if !ocContainsArg(args, "--format") {
		t.Error("--format should be present")
	}
}

func TestOpenCodeBackend_BuildArgs_WithModel(t *testing.T) {
	b := &OpenCodeBackend{}
	args := b.BuildArgs(SpawnRequest{
		Prompt: "test",
		Model:  "anthropic/claude-sonnet-4-6",
	})
	if !ocContainsArg(args, "--model") {
		t.Error("--model should be present")
	}
	for i, a := range args {
		if a == "--model" && i+1 < len(args) {
			if args[i+1] != "anthropic/claude-sonnet-4-6" {
				t.Errorf("expected model value, got %s", args[i+1])
			}
		}
	}
}

func TestOpenCodeBackend_BuildArgs_WithSession(t *testing.T) {
	b := &OpenCodeBackend{}
	args := b.BuildArgs(SpawnRequest{
		Prompt:    "test",
		SessionID: "ses_abc",
	})
	if !ocContainsArg(args, "--session") {
		t.Error("--session should be present")
	}
}

func TestOpenCodeBackend_BuildArgs_WithMaxTurns(t *testing.T) {
	b := &OpenCodeBackend{}
	args := b.BuildArgs(SpawnRequest{
		Prompt:   "test",
		MaxTurns: 10,
	})
	if !ocContainsArg(args, "--max-turns") {
		t.Error("--max-turns should be present")
	}
}

func TestOpenCodeBackend_BuildArgs_WithDisallowedTools(t *testing.T) {
	b := &OpenCodeBackend{}
	args := b.BuildArgs(SpawnRequest{
		Prompt:          "test",
		DisallowedTools: []string{"Bash", "Write"},
	})
	if !ocContainsArg(args, "--excludedTools") {
		t.Error("--excludedTools should be present")
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "Bash,Write") {
		t.Error("excluded tools should be comma-joined")
	}
}

func TestOpenCodeBackend_ConvertEvent_Text(t *testing.T) {
	b := &OpenCodeBackend{}
	ev := b.convertEvent(&ocEvent{Type: "text", Text: "hello"})
	if ev.Type != "assistant" {
		t.Errorf("expected assistant, got %s", ev.Type)
	}
	if ev.Text != "hello" {
		t.Errorf("expected hello, got %s", ev.Text)
	}
}

func TestOpenCodeBackend_ConvertEvent_ToolUse(t *testing.T) {
	b := &OpenCodeBackend{}
	ev := b.convertEvent(&ocEvent{
		Type:     "tool_use",
		ToolName: "Read",
		ToolInput: map[string]any{
			"file_path": "/tmp/test",
		},
	})
	if ev.Type != "assistant" {
		t.Errorf("expected assistant, got %s", ev.Type)
	}
	if ev.ToolName != "Read" {
		t.Errorf("expected Read, got %s", ev.ToolName)
	}
}

func TestOpenCodeBackend_ConvertEvent_StepFinish(t *testing.T) {
	b := &OpenCodeBackend{}
	ev := b.convertEvent(&ocEvent{Type: "step_finish", Result: "done"})
	if ev.Type != "result" {
		t.Errorf("expected result, got %s", ev.Type)
	}
	if ev.Result != "done" {
		t.Errorf("expected done, got %s", ev.Result)
	}
}

func TestOpenCodeBackend_ConvertEvent_Error(t *testing.T) {
	b := &OpenCodeBackend{}
	ev := b.convertEvent(&ocEvent{Type: "error", ErrorMsg: "failed"})
	if ev.Type != "error" {
		t.Errorf("expected error, got %s", ev.Type)
	}
	if !ev.IsError {
		t.Error("IsError should be true")
	}
	if ev.Text != "failed" {
		t.Errorf("expected failed, got %s", ev.Text)
	}
}

func TestOpenCodeBackend_ConvertEvent_System(t *testing.T) {
	b := &OpenCodeBackend{}
	ev := b.convertEvent(&ocEvent{Type: "step_start"})
	if ev.Type != "system" {
		t.Errorf("expected system, got %s", ev.Type)
	}
	if ev.Subtype != "step_start" {
		t.Errorf("expected step_start subtype, got %s", ev.Subtype)
	}
}

func ocContainsArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}
