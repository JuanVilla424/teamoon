package backend

import "context"

// Capabilities declares what optional features a backend supports.
type Capabilities struct {
	JSONStreaming  bool
	SessionResume bool
	MCPConfig     bool
	ToolFiltering bool
}

// SpawnRequest is the unified input to Execute.
type SpawnRequest struct {
	Prompt    string
	ProjectDir string
	WorkDir    string
	AddDirs    []string

	SessionID string
	Model     string
	Effort    string
	MaxTurns  int
	MCPConfig string // path to temp MCP JSON file
	Env       []string

	AllowedTools    []string
	DisallowedTools []string

	Phase          string // "exec", "plan", "chat", "job"
	PermissionMode string // "plan" for --permission-mode plan; empty = omit
}

// Event is a normalized representation of one logical event from a backend.
type Event struct {
	Type      string // "assistant", "user", "result", "error", "system", "text_delta"
	Subtype   string
	SessionID string
	Text      string
	ToolName  string
	ToolInput map[string]any
	IsError   bool
	Result    string
	Denials   []string
	ToolsUsed []string
}

// SpawnResult is the outcome of a completed Execute call.
type SpawnResult struct {
	ExitCode  int
	Output    string
	Denials   []string
	ToolsUsed []string
	SessionID string
}

// Backend is the core interface every coding CLI adapter must implement.
type Backend interface {
	// Name returns the unique identifier for this backend ("claude", "opencode", etc.)
	Name() string

	// Caps returns the backend's declared capabilities.
	Caps() Capabilities

	// Available returns nil if the backend binary is found and usable.
	Available() error

	// Execute spawns the backend, streams normalized Events to the channel,
	// and returns when the process exits or ctx is cancelled.
	// The events channel is closed by Execute before it returns.
	Execute(ctx context.Context, req SpawnRequest, events chan<- Event) (SpawnResult, error)

	// ResolveModel translates meta-model names to backend-specific model IDs.
	// phase is "plan", "exec", "chat", or "job".
	ResolveModel(model, phase string) string
}
