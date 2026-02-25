package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
}

type SpawnConfig struct {
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	MaxTurns       int    `json:"max_turns"`
	StepTimeoutMin int    `json:"step_timeout_min"`
}

type SkeletonConfig struct {
	WebSearch   bool `json:"web_search"`
	BuildVerify bool `json:"build_verify"`
	Test           bool `json:"test"`
	PreCommit      bool `json:"pre_commit"`
	Commit         bool `json:"commit"`
	Push           bool `json:"push"`
}

func DefaultSkeleton() SkeletonConfig {
	return SkeletonConfig{
		WebSearch:   true,
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
		Spawn:              SpawnConfig{Model: "opusplan", Effort: "high", MaxTurns: 15, StepTimeoutMin: 4},
		Skeleton:           DefaultSkeleton(),
		MaxConcurrent:      3,
		AutopilotAutostart: false,
		MCPServers:         nil,
		SourceDir:          filepath.Join(home, "Projects", "teamoon"),
	}
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

// InitMCPFromGlobal populates MCPServers from global settings if nil (one-time bootstrap).
func InitMCPFromGlobal(cfg *Config) {
	if cfg.MCPServers != nil {
		return
	}
	cfg.MCPServers = ReadGlobalMCPServers()
	// Attach known skeleton steps
	for name, mcp := range cfg.MCPServers {
		if step, ok := KnownSkeletonSteps[name]; ok && mcp.SkeletonStep == nil {
			mcp.SkeletonStep = &step
			cfg.MCPServers[name] = mcp
		}
	}
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
