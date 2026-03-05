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

	"github.com/JuanVilla424/teamoon/internal/config"
)

// ClaudeBackend implements Backend for the Claude Code CLI.
type ClaudeBackend struct{}

func (b *ClaudeBackend) Name() string { return "claude" }

func (b *ClaudeBackend) Caps() Capabilities {
	return Capabilities{
		JSONStreaming:  true,
		SessionResume: true,
		MCPConfig:     true,
		ToolFiltering: true,
	}
}

func (b *ClaudeBackend) Available() error {
	_, err := exec.LookPath("claude")
	return err
}

// ResolveModel maps teamoon meta-models to full Claude CLI model IDs.
func (b *ClaudeBackend) ResolveModel(model, phase string) string {
	if model == "opusplan" {
		if phase == "plan" || phase == "chat" {
			return "claude-opus-4-6"
		}
		return "claude-sonnet-4-6"
	}
	switch model {
	case "opus":
		return "claude-opus-4-6"
	case "sonnet":
		return "claude-sonnet-4-6"
	case "haiku":
		return "claude-haiku-4-5-20251001"
	}
	return model
}

// BuildArgs assembles Claude CLI arguments from a SpawnRequest.
// Returns the args slice and an optional cleanup function (for temp files).
func (b *ClaudeBackend) BuildArgs(req SpawnRequest) ([]string, func()) {
	var args []string
	if req.SessionID != "" {
		args = []string{
			"--resume", req.SessionID,
			"-p", req.Prompt,
			"--output-format", "stream-json",
			"--verbose",
		}
	} else {
		args = []string{
			"-p", req.Prompt,
			"--output-format", "stream-json",
			"--verbose",
			"--no-session-persistence",
		}
	}

	if req.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(req.MaxTurns))
	} else if req.MaxTurns < 0 {
		args = append(args, "--max-turns", "15")
	}

	if os.Getuid() != 0 {
		args = append(args, "--dangerously-skip-permissions")
	} else if len(req.AllowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, req.AllowedTools...)
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}
	for _, dir := range req.AddDirs {
		args = append(args, "--add-dir", dir)
	}
	if len(req.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools",
			strings.Join(req.DisallowedTools, ","))
	}

	if req.PermissionMode != "" {
		args = append(args, "--permission-mode", req.PermissionMode)
	}

	var cleanup func()
	if req.MCPConfig != "" {
		args = append(args, "--mcp-config", req.MCPConfig)
	}

	return args, cleanup
}

// Execute spawns the Claude CLI, parses stream-json, and emits normalized Events.
func (b *ClaudeBackend) Execute(ctx context.Context, req SpawnRequest, events chan<- Event) (SpawnResult, error) {
	defer close(events)

	args, cleanup := b.BuildArgs(req)
	if cleanup != nil {
		defer cleanup()
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

	// Set environment
	if len(req.Env) > 0 {
		cmd.Env = req.Env
	}

	// Set working directory
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
	var denials []string
	var toolsUsed []string
	var capturedSessionID string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fullOutput.WriteString(line + "\n")

		var raw StreamEvent
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// Capture session_id
		if raw.SessionID != "" && capturedSessionID == "" {
			capturedSessionID = raw.SessionID
		}

		// Convert to normalized Event and emit
		ev := b.convertEvent(&raw)
		if ev.SessionID == "" && capturedSessionID != "" {
			ev.SessionID = capturedSessionID
		}
		events <- ev

		// Track tools and denials
		switch raw.Type {
		case "assistant":
			if raw.Message != nil {
				for _, c := range raw.Message.Content {
					if c.Type == "tool_use" && c.Name != "" {
						toolsUsed = append(toolsUsed, c.Name)
					}
				}
			}
		case "result":
			for _, d := range raw.PermissionDenials {
				denials = append(denials, d.ToolName)
			}
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
		Denials:   denials,
		ToolsUsed: toolsUsed,
		SessionID: capturedSessionID,
	}, nil
}

// convertEvent translates a Claude stream-json event to a normalized Event.
func (b *ClaudeBackend) convertEvent(raw *StreamEvent) Event {
	ev := Event{
		Type:      raw.Type,
		Subtype:   raw.Subtype,
		SessionID: raw.SessionID,
		IsError:   raw.IsError,
	}

	switch raw.Type {
	case "assistant":
		if raw.Message != nil {
			var texts []string
			for _, c := range raw.Message.Content {
				switch c.Type {
				case "tool_use":
					ev.ToolName = c.Name
					ev.ToolInput = c.Input
				case "text":
					if c.Text != "" {
						texts = append(texts, c.Text)
					}
				}
			}
			ev.Text = strings.Join(texts, "\n")
		}
	case "user":
		if raw.Message != nil {
			var texts []string
			for _, c := range raw.Message.Content {
				if c.Type == "tool_result" {
					content := c.Content
					if content == "" && raw.ToolUseResult != nil {
						content = raw.ToolUseResult.Stdout
					}
					if content != "" {
						texts = append(texts, content)
					}
				}
			}
			ev.Text = strings.Join(texts, "\n")
		}
	case "result":
		ev.Result = raw.Result
		for _, d := range raw.PermissionDenials {
			ev.Denials = append(ev.Denials, d.ToolName)
		}
	case "error":
		if raw.Error != nil {
			ev.Text = raw.Error.Message
			ev.IsError = true
		}
	case "system":
		ev.Text = raw.Subtype
	}

	return ev
}

// --- Claude-specific stream-json types (private) ---

// StreamEvent represents a single line from Claude CLI's stream-json output.
// Exported for backward compatibility with handlers that parse Claude output directly.
type StreamEvent struct {
	Type              string             `json:"type"`
	Subtype           string             `json:"subtype,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
	Message           *StreamMessage     `json:"message,omitempty"`
	Result            string             `json:"result,omitempty"`
	Error             *StreamError       `json:"error,omitempty"`
	IsError           bool               `json:"is_error,omitempty"`
	PermissionDenials []PermissionDenial `json:"permission_denials,omitempty"`
	ToolUseResult     *ToolUseResult     `json:"tool_use_result,omitempty"`
}

type StreamMessage struct {
	Content []StreamContent `json:"content"`
}

type StreamContent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
}

type StreamError struct {
	Message string `json:"message"`
}

type PermissionDenial struct {
	ToolName string `json:"tool_name"`
}

type ToolUseResult struct {
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

// --- Exported helpers used by engine for log formatting ---

// FormatStreamEvent converts a raw Claude StreamEvent into human-readable console lines.
// Used by handlers that still parse Claude's stream-json directly.
func FormatStreamEvent(event StreamEvent) string {
	var lines []string

	switch event.Type {
	case "assistant":
		if event.Message != nil {
			for _, c := range event.Message.Content {
				switch c.Type {
				case "tool_use":
					desc := FormatToolCall(c.Name, c.Input)
					lines = append(lines, "⏺ "+desc)
				case "text":
					if c.Text != "" {
						lines = append(lines, c.Text)
					}
				}
			}
		}
	case "user":
		if event.Message != nil {
			for _, c := range event.Message.Content {
				if c.Type == "tool_result" {
					content := c.Content
					if content == "" && event.ToolUseResult != nil {
						content = event.ToolUseResult.Stdout
					}
					if content != "" {
						if len(content) > 500 {
							content = content[:500] + "\n... (truncated)"
						}
						prefix := "  ↳ "
						if c.IsError {
							prefix = "  ✗ "
						}
						lines = append(lines, prefix+content)
					}
				}
			}
		}
	case "result":
		// result is captured separately; don't duplicate
	case "error":
		if event.Error != nil {
			lines = append(lines, "✗ Error: "+event.Error.Message)
		}
	case "system":
		if event.Subtype != "" {
			lines = append(lines, "⚙ "+event.Subtype)
		}
	}

	return strings.Join(lines, "\n")
}

// FormatEvent converts a normalized Event into human-readable console lines.
func FormatEvent(ev Event) string {
	switch ev.Type {
	case "assistant":
		var lines []string
		if ev.ToolName != "" {
			lines = append(lines, "⏺ "+FormatToolCall(ev.ToolName, ev.ToolInput))
		}
		if ev.Text != "" {
			lines = append(lines, ev.Text)
		}
		return strings.Join(lines, "\n")
	case "user":
		if ev.Text != "" {
			content := ev.Text
			if len(content) > 500 {
				content = content[:500] + "\n... (truncated)"
			}
			prefix := "  ↳ "
			if ev.IsError {
				prefix = "  ✗ "
			}
			return prefix + content
		}
	case "result":
		// result is captured separately; don't duplicate
	case "error":
		if ev.Text != "" {
			return "✗ Error: " + ev.Text
		}
	case "system":
		if ev.Subtype != "" {
			return "⚙ " + ev.Subtype
		}
	}
	return ""
}

// FormatToolCall produces a human-readable summary of a tool invocation.
func FormatToolCall(name string, input map[string]any) string {
	str := func(key string) string {
		if v, ok := input[key]; ok {
			if s, ok := v.(string); ok {
				if len(s) > 120 {
					return s[:120] + "..."
				}
				return s
			}
		}
		return ""
	}
	switch name {
	case "Read":
		if p := str("file_path"); p != "" {
			return "Read " + p
		}
	case "Write":
		if p := str("file_path"); p != "" {
			return "Write " + p
		}
	case "Edit":
		if p := str("file_path"); p != "" {
			return "Edit " + p
		}
	case "Glob":
		if p := str("pattern"); p != "" {
			return "Glob " + p
		}
	case "Grep":
		if p := str("pattern"); p != "" {
			path := str("path")
			if path != "" {
				return "Grep " + p + " in " + path
			}
			return "Grep " + p
		}
	case "Bash":
		if c := str("command"); c != "" {
			return "Bash: " + c
		}
	case "WebSearch":
		if q := str("query"); q != "" {
			return "WebSearch: " + q
		}
	case "WebFetch":
		if u := str("url"); u != "" {
			return "WebFetch: " + u
		}
	case "Task":
		if d := str("description"); d != "" {
			return "Task: " + d
		}
	case "TodoWrite":
		return "TodoWrite"
	case "Skill":
		if s := str("skill"); s != "" {
			return "Skill: " + s
		}
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.SplitN(name, "__", 3)
		if len(parts) >= 3 {
			return parts[1] + ": " + strings.ReplaceAll(parts[2], "-", " ")
		}
	}
	return name
}

// ExtractResult scans raw Claude stream-json output for the result event.
func ExtractResult(rawOutput string) string {
	for _, line := range strings.Split(rawOutput, "\n") {
		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "result" && event.Result != "" {
			r := event.Result
			if len(r) > 300 {
				r = r[:300] + "..."
			}
			return r
		}
	}
	return ""
}

// FilterEnv removes environment variables matching the given key prefix.
func FilterEnv(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// BuildMCPArgs constructs the MCP-related portion of a SpawnRequest from config.
// Returns the temp file path (for MCPConfig) and the allowed MCP tool names for root mode.
func BuildMCPArgs(cfg config.Config) (mcpConfig string, mcpTools []string, cleanup func()) {
	if cfg.MCPServers == nil {
		return "", nil, nil
	}
	essential := make(map[string]config.MCPServer)
	var tools []string
	for name, s := range cfg.MCPServers {
		if s.Enabled && (name == "context7" || name == "github" || s.SkeletonStep != nil) {
			essential[name] = s
			tools = append(tools, "mcp__"+name)
		}
	}
	if len(essential) == 0 {
		return "", nil, nil
	}
	tmpPath, err := config.BuildMCPConfigJSON(essential)
	if err != nil {
		return "", tools, nil
	}
	return tmpPath, tools, func() { os.Remove(tmpPath) }
}
