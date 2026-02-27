package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Plugin represents an installed Claude Code plugin.
type Plugin struct {
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Enabled     bool   `json:"enabled"`
}

// DefaultPlugin defines a plugin to install during onboarding.
type DefaultPlugin struct {
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"` // GitHub repo source (e.g., "thedotmack/claude-mem")
	Description string `json:"description"`
}

// DefaultPlugins lists plugins installed during onboarding.
var DefaultPlugins = []DefaultPlugin{
	// LSPs
	{"typescript-lsp", "claude-plugins-official", "TypeScript/JavaScript language server"},
	{"pyright-lsp", "claude-plugins-official", "Python type checking and language server"},
	{"gopls-lsp", "claude-plugins-official", "Go language server"},
	{"rust-analyzer-lsp", "claude-plugins-official", "Rust language server"},
	{"clangd-lsp", "claude-plugins-official", "C/C++ language server"},
	// Enhancement plugins
	{"hookify", "claude-plugins-official", "Create hooks to prevent unwanted behaviors"},
	{"security-guidance", "claude-plugins-official", "Security best practices guidance"},
	{"pr-review-toolkit", "claude-plugins-official", "Comprehensive PR review toolkit"},
	{"claude-code-setup", "claude-plugins-official", "Claude Code setup automation"},
	{"code-simplifier", "claude-plugins-official", "Simplify code for clarity and maintainability"},
	{"feature-dev", "claude-plugins-official", "Guided feature development with codebase understanding"},
	// Third-party
	{"claude-mem", "thedotmack/claude-mem", "Persistent memory across Claude sessions"},
}

// ReadInstalled parses ~/.claude/settings.json enabledPlugins and returns installed plugins.
func ReadInstalled() []Plugin {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var plugins []Plugin
	for key, enabled := range raw.EnabledPlugins {
		name, marketplace := parsePluginKey(key)
		plugins = append(plugins, Plugin{
			Name:        name,
			Marketplace: marketplace,
			Enabled:     enabled,
		})
	}
	return plugins
}

// IsInstalled checks if a plugin with the given name is in enabledPlugins.
func IsInstalled(name string) bool {
	for _, p := range ReadInstalled() {
		if p.Name == name {
			return true
		}
	}
	return false
}

// Install adds a marketplace and installs a plugin via claude CLI.
func Install(name, marketplace string) error {
	// Step 1: Add marketplace (idempotent)
	if marketplace != "" {
		cmd := exec.Command("claude", "plugin", "marketplace", "add", marketplace)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Ignore error — marketplace may already exist
		cmd.Run()
	}

	// Step 2: Install plugin
	cmd := exec.Command("claude", "plugin", "install", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing plugin %s: %w: %s", name, err, string(out))
	}
	return nil
}

// Uninstall removes a plugin via claude CLI.
func Uninstall(name string) error {
	cmd := exec.Command("claude", "plugin", "uninstall", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstalling plugin %s: %w: %s", name, err, string(out))
	}
	return nil
}

// parsePluginKey splits "name@marketplace" into name and marketplace.
func parsePluginKey(key string) (string, string) {
	parts := strings.SplitN(key, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}
