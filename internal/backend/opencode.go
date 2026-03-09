package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// OpenCodeBackend implements Backend for the opencode CLI.
type OpenCodeBackend struct{}

func (b *OpenCodeBackend) Name() string { return "opencode" }

func (b *OpenCodeBackend) Caps() Capabilities {
	return Capabilities{
		JSONStreaming:  true,
		SessionResume: true,
		MCPConfig:     false, // config-based only, no per-invocation flag
		ToolFiltering: true,
	}
}

func (b *OpenCodeBackend) Available() error {
	_, err := exec.LookPath("opencode")
	return err
}

// ResolveModel maps teamoon meta-models to opencode provider/model IDs.
func (b *OpenCodeBackend) ResolveModel(model, phase string) string {
	if model == "opusplan" {
		if phase == "plan" || phase == "chat" {
			return "anthropic/claude-opus-4-6"
		}
		return "anthropic/claude-sonnet-4-6"
	}
	switch model {
	case "opus":
		return "anthropic/claude-opus-4-6"
	case "sonnet":
		return "anthropic/claude-sonnet-4-6"
	case "haiku":
		return "anthropic/claude-haiku-4-5-20251001"
	}
	return model
}

// BuildArgs assembles opencode CLI arguments from a SpawnRequest.
func (b *OpenCodeBackend) BuildArgs(req SpawnRequest) []string {
	args := []string{"run", req.Prompt, "--format", "json"}

	if req.SessionID != "" {
		args = append(args, "--session", req.SessionID)
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	if req.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(req.MaxTurns))
	}

	if os.Getuid() != 0 {
		args = append(args, "--yolo")
	}

	if len(req.DisallowedTools) > 0 {
		args = append(args, "--excludedTools",
			strings.Join(req.DisallowedTools, ","))
	}

	if len(req.AllowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, req.AllowedTools...)
	}

	return args
}

// Execute spawns the opencode CLI, parses NDJSON events, and emits normalized Events.
func (b *OpenCodeBackend) Execute(ctx context.Context, req SpawnRequest, events chan<- Event) (SpawnResult, error) {
	defer close(events)

	args := b.BuildArgs(req)
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}

	if len(req.Env) > 0 {
		cmd.Env = req.Env
	}

	workDir := req.WorkDir
	if workDir == "" {
		workDir = req.ProjectDir
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	devNull, _ := os.Open(os.DevNull)
	if devNull != nil {
		cmd.Stdin = devNull
		defer devNull.Close()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SpawnResult{ExitCode: -1}, err
	}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return SpawnResult{ExitCode: -1}, err
	}

	var fullOutput strings.Builder
	var toolsUsed []string
	var capturedSessionID string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fullOutput.WriteString(line + "\n")

		var raw ocEvent
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		if raw.SessionID != "" && capturedSessionID == "" {
			capturedSessionID = raw.SessionID
		}

		ev := b.convertEvent(&raw)
		if ev.SessionID == "" && capturedSessionID != "" {
			ev.SessionID = capturedSessionID
		}
		events <- ev

		if raw.Type == "tool_use" && raw.ToolName != "" {
			toolsUsed = append(toolsUsed, raw.ToolName)
		}
	}

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return SpawnResult{ExitCode: -1, Output: fullOutput.String()}, err
		}
	}

	if stderrBuf.Len() > 0 {
		fullOutput.WriteString("\n[stderr] " + stderrBuf.String())
	}

	return SpawnResult{
		ExitCode:  exitCode,
		Output:    fullOutput.String(),
		ToolsUsed: toolsUsed,
		SessionID: capturedSessionID,
	}, nil
}

// convertEvent translates an opencode NDJSON event to a normalized Event.
func (b *OpenCodeBackend) convertEvent(raw *ocEvent) Event {
	ev := Event{
		SessionID: raw.SessionID,
	}

	switch raw.Type {
	case "text":
		ev.Type = "assistant"
		ev.Text = raw.Text
	case "tool_use":
		ev.Type = "assistant"
		ev.ToolName = raw.ToolName
		ev.ToolInput = raw.ToolInput
	case "tool_result":
		ev.Type = "user"
		ev.Text = raw.Content
		ev.IsError = raw.IsError
	case "step_finish", "result":
		ev.Type = "result"
		ev.Result = raw.Result
	case "error":
		ev.Type = "error"
		ev.Text = raw.ErrorMsg
		ev.IsError = true
	case "step_start", "reasoning":
		ev.Type = "system"
		ev.Subtype = raw.Type
	default:
		ev.Type = "system"
		ev.Subtype = raw.Type
	}

	return ev
}

// --- opencode-specific NDJSON types (private) ---

type ocEvent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
	Content   string         `json:"content,omitempty"`
	Result    string         `json:"result,omitempty"`
	ErrorMsg  string         `json:"error,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
}
