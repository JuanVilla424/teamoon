package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type SkeletonStep struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	ReadOnly    bool   `json:"read_only"`
}

type MCPServer struct {
	Command      string        `json:"command"`
	Args         []string      `json:"args"`
	Enabled      bool          `json:"enabled"`
	SkeletonStep *SkeletonStep `json:"skeleton_step,omitempty"`
}

// KnownSkeletonSteps maps MCP names to their default skeleton steps.
// When an MCP is installed and matches a known name, the step is auto-attached.
var KnownSkeletonSteps = map[string]SkeletonStep{
	"context7": {
		Label:       "Context7 Lookup",
		Description: "Look up library documentation",
		Prompt:      "Use resolve-library-id then query-docs for each relevant library. Note relevant APIs and patterns.",
		ReadOnly:    true,
	},
	"chrome-devtools": {
		Label:       "Browser Verification",
		Description: "Verify UI in browser using Chrome DevTools",
		Prompt:      "Open the app in the browser, take a snapshot, verify the UI renders correctly and matches expectations. Check for console errors.",
		ReadOnly:    false,
	},
}

// SkeletonPhase represents a single phase in the task execution skeleton.
// When Phases is set in SkeletonConfig, each phase is fully defined here.
type SkeletonPhase struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Prompt   string `json:"prompt"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Generate bool   `json:"generate,omitempty"`
	Enabled  bool   `json:"enabled"`
}

type SpawnConfig struct {
	Model           string `json:"model"`
	Effort          string `json:"effort"`
	MaxTurns        int    `json:"max_turns"`
	StepTimeoutMin  int    `json:"step_timeout_min"`
	PlanTimeoutMin  int    `json:"plan_timeout_min"`   // 0 = use default (15 min)
	PlanMaxTurns    int    `json:"plan_max_turns"`     // 0 = unlimited (no --max-turns flag)
	MaxPlanAttempts int    `json:"max_plan_attempts"`  // 0 falls back to default of 3
}

type SkeletonConfig struct {
	WebSearch   bool `json:"web_search"`
	DocSetup    bool `json:"doc_setup"`
	BuildVerify bool `json:"build_verify"`
	Test           bool `json:"test"`
	PreCommit      bool `json:"pre_commit"`
	Commit         bool `json:"commit"`
	Push           bool `json:"push"`
}

func DefaultSkeleton() SkeletonConfig {
	return SkeletonConfig{
		WebSearch:   true,
		DocSetup:    true,
		BuildVerify: true,
		Test:           true,
		PreCommit:      true,
		Commit:         true,
		Push:           false,
	}
}

// SkeletonFor returns the skeleton config for a project, falling back to global.
func SkeletonFor(cfg Config, project string) SkeletonConfig {
	if cfg.ProjectSkeletons != nil {
		if sk, ok := cfg.ProjectSkeletons[project]; ok {
			return sk
		}
	}
	return cfg.Skeleton
}

type Config struct {
	ProjectsDir        string                `json:"projects_dir"`
	ClaudeDir          string                `json:"claude_dir"`
	RefreshIntervalSec int                   `json:"refresh_interval_sec"`
	ContextLimit       int                   `json:"context_limit"`
	WebEnabled         bool                  `json:"web_enabled"`
	WebPort            int                   `json:"web_port"`
	WebHost            string                `json:"web_host"`
	WebPassword        string                `json:"web_password"`
	WebhookURL         string                `json:"webhook_url,omitempty"`
	Spawn              SpawnConfig                    `json:"spawn"`
	Skeleton           SkeletonConfig                 `json:"skeleton"`
	ProjectSkeletons   map[string]SkeletonConfig      `json:"project_skeletons,omitempty"`
	MaxConcurrent      int                            `json:"max_concurrent"`
	AutopilotAutostart bool                           `json:"autopilot_autostart"`
	MCPServers         map[string]MCPServer           `json:"mcp_servers,omitempty"`
	SourceDir          string                         `json:"source_dir,omitempty"`
	Debug              bool                           `json:"debug,omitempty"`
	LogRetentionDays   int                            `json:"log_retention_days"`
	SudoEnabled        bool                           `json:"sudo_enabled,omitempty"`
	PhaseHints         map[string]string              `json:"phase_hints,omitempty"`
}

// DefaultPhaseHints returns descriptions for each skeleton phase.
// These hints are included in the skeleton JSON so the LLM knows what each phase means.
func DefaultPhaseHints() map[string]string {
	return map[string]string{
		"doc_setup":    "MUST be the FIRST implementation step. Read CLAUDE.md, README.md. Create or update ARCHITECT.md with project architecture, patterns, conventions. Update CONTEXT.md.",
		"web_search":   "Search the web for current best practices and documentation.",
		"build_verify": "Compile/build the project. Create build tooling if missing. Verify clean build.",
		"test":         "Run existing tests. Create new tests for changes. Set up test infra if missing.",
		"pre_commit":   "Run linters/formatters. Install pre-commit hooks if missing.",
		"commit":       "Single git commit: type(core): description. No emojis, no Co-Authored-By.",
		"push":         "Push to remote repository.",
	}
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ProjectsDir:        filepath.Join(home, "Projects"),
		ClaudeDir:          filepath.Join(home, ".claude"),
		RefreshIntervalSec: 30,
		ContextLimit:       0,
		WebEnabled:         false,
		WebPort:            7777,
		WebHost:            "",
		WebPassword:        "",
		Spawn:              SpawnConfig{Model: "opusplan", Effort: "high", MaxTurns: 15, StepTimeoutMin: 4, PlanMaxTurns: 15, MaxPlanAttempts: 3},
		Skeleton:           DefaultSkeleton(),
		MaxConcurrent:      3,
		AutopilotAutostart: false,
		MCPServers:         nil,
		SourceDir:          filepath.Join(home, "Projects", "teamoon"),
		LogRetentionDays:   20,
		PhaseHints:         DefaultPhaseHints(),
	}
}

// IsPasswordHashed returns true if the password string is a bcrypt hash.
func IsPasswordHashed(pw string) bool {
	return strings.HasPrefix(pw, "$2a$") || strings.HasPrefix(pw, "$2b$")
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamoon")
}

func Load() (Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(ConfigDir(), "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if saveErr := Save(cfg); saveErr != nil {
				return cfg, saveErr
			}
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	// Backfill missing phase hints for existing configs
	defaults := DefaultPhaseHints()
	if cfg.PhaseHints == nil {
		cfg.PhaseHints = defaults
	} else {
		for k, v := range defaults {
			if _, ok := cfg.PhaseHints[k]; !ok {
				cfg.PhaseHints[k] = v
			}
		}
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}

// ReadGlobalMCPServers reads MCP servers from ~/.claude/settings.json.
func ReadGlobalMCPServers() map[string]MCPServer {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]MCPServer, len(raw.MCPServers))
	for name, s := range raw.MCPServers {
		result[name] = MCPServer{Command: s.Command, Args: s.Args, Enabled: true}
	}
	return result
}

// ReadGlobalMCPServersFrom reads MCP servers from a specific file path (for testing).
func ReadGlobalMCPServersFrom(path string) map[string]MCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]MCPServer, len(raw.MCPServers))
	for name, s := range raw.MCPServers {
		result[name] = MCPServer{Command: s.Command, Args: s.Args, Enabled: true}
	}
	return result
}

// BuildMCPConfigJSON writes enabled MCP servers to a temp JSON file and returns the path.
func BuildMCPConfigJSON(servers map[string]MCPServer) (string, error) {
	type mcpEntry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	out := struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}{
		MCPServers: make(map[string]mcpEntry, len(servers)),
	}
	for name, s := range servers {
		out.MCPServers[name] = mcpEntry{Command: s.Command, Args: s.Args}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "teamoon-mcp-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// AttachKnownSkeletonSteps ensures every MCP with a known skeleton step has it set.
// Returns true if any step was attached (config needs saving).
func AttachKnownSkeletonSteps(servers map[string]MCPServer) bool {
	changed := false
	for name, mcp := range servers {
		if step, ok := KnownSkeletonSteps[name]; ok && mcp.SkeletonStep == nil {
			mcp.SkeletonStep = &step
			servers[name] = mcp
			changed = true
		}
	}
	return changed
}

// InitMCPFromGlobal populates MCPServers from global settings if nil (one-time bootstrap).
// Persists config if skeleton steps were attached to existing servers.
func InitMCPFromGlobal(cfg *Config) {
	if cfg.MCPServers != nil {
		if AttachKnownSkeletonSteps(cfg.MCPServers) {
			Save(*cfg)
		}
		return
	}
	cfg.MCPServers = ReadGlobalMCPServers()
	AttachKnownSkeletonSteps(cfg.MCPServers)
}

// RemoveMCPFromGlobal removes an MCP server entry from ~/.claude/settings.json.
func RemoveMCPFromGlobal(name string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	delete(servers, name)

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// InstallMCPToGlobal adds an MCP server entry to ~/.claude/settings.json.
// It reads the file, merges the new server into mcpServers, and writes back.
// If envVars is non-empty, they are set in the "env" field of the server entry.
func InstallMCPToGlobal(name, command string, args []string, envVars map[string]string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")

	// Read existing file (or start fresh)
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	// Parse existing mcpServers
	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	entry := mcpEntry{Command: command, Args: args}
	if len(envVars) > 0 {
		entry.Env = envVars
	}
	servers[name] = entry

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// InstallMCPToGlobalAt is like InstallMCPToGlobal but writes to a specific path (for testing).
func InstallMCPToGlobalAt(path, name, command string, args []string, envVars map[string]string) error {
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	entry := mcpEntry{Command: command, Args: args}
	if len(envVars) > 0 {
		entry.Env = envVars
	}
	servers[name] = entry

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// planMCPAllowlist contains MCP servers suitable for plan generation (lightweight, read-only).
var planMCPAllowlist = map[string]bool{"context7": true}

// FilterPlanMCP returns only lightweight MCP servers suitable for plan generation.
// Excludes heavy servers like chrome-devtools (spawns Chrome browser) and others
// that add startup overhead without benefiting read-only investigation.
func FilterPlanMCP(servers map[string]MCPServer) map[string]MCPServer {
	result := make(map[string]MCPServer)
	for name, s := range servers {
		if s.Enabled && planMCPAllowlist[name] {
			result[name] = s
		}
	}
	return result
}

// FilterEnabledMCP returns only enabled servers from the map.
func FilterEnabledMCP(servers map[string]MCPServer) map[string]MCPServer {
	result := make(map[string]MCPServer)
	for name, s := range servers {
		if s.Enabled {
			result[name] = s
		}
	}
	return result
}
