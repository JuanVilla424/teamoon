package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !strings.Contains(cfg.ProjectsDir, "Projects") {
		t.Errorf("ProjectsDir should contain 'Projects', got %s", cfg.ProjectsDir)
	}
	if cfg.WebPort != 7777 {
		t.Errorf("WebPort should be 7777, got %d", cfg.WebPort)
	}
	if cfg.RefreshIntervalSec != 30 {
		t.Errorf("RefreshIntervalSec should be 30, got %d", cfg.RefreshIntervalSec)
	}
	if cfg.Spawn.MaxTurns != 15 {
		t.Errorf("Spawn.MaxTurns should be 15, got %d", cfg.Spawn.MaxTurns)
	}
	if cfg.WebHost != "localhost" {
		t.Errorf("WebHost should be 'localhost', got %s", cfg.WebHost)
	}
	if cfg.Spawn.Model != "" {
		t.Errorf("Spawn.Model should be empty, got %s", cfg.Spawn.Model)
	}
	if cfg.MCPServers != nil {
		t.Error("MCPServers should be nil by default")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")

	cfg := DefaultConfig()
	cfg.ProjectsDir = "/tmp/test-projects"
	cfg.WebPort = 9999
	cfg.Spawn = SpawnConfig{Model: "sonnet", Effort: "high", MaxTurns: 50}
	cfg.MCPServers = map[string]MCPServer{
		"context7": {Command: "npx", Args: []string{"-y", "@context7/mcp"}, Enabled: true},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	var loaded Config
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.ProjectsDir != "/tmp/test-projects" {
		t.Errorf("ProjectsDir mismatch: %s", loaded.ProjectsDir)
	}
	if loaded.WebPort != 9999 {
		t.Errorf("WebPort mismatch: %d", loaded.WebPort)
	}
	if loaded.Spawn.Model != "sonnet" {
		t.Errorf("Spawn.Model mismatch: %s", loaded.Spawn.Model)
	}
	if loaded.Spawn.MaxTurns != 50 {
		t.Errorf("Spawn.MaxTurns mismatch: %d", loaded.Spawn.MaxTurns)
	}
	srv, ok := loaded.MCPServers["context7"]
	if !ok {
		t.Fatal("MCPServers missing context7")
	}
	if srv.Command != "npx" {
		t.Errorf("MCPServer command mismatch: %s", srv.Command)
	}
	if !srv.Enabled {
		t.Error("MCPServer should be enabled")
	}
}

func TestReadGlobalMCPServersFrom(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	content := `{
		"mcpServers": {
			"context7": {
				"command": "npx",
				"args": ["-y", "@upstash/context7-mcp@latest"]
			},
			"memory": {
				"command": "npx",
				"args": ["-y", "@anthropic/memory-mcp"]
			}
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	servers := ReadGlobalMCPServersFrom(path)
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	ctx7, ok := servers["context7"]
	if !ok {
		t.Fatal("missing context7 server")
	}
	if ctx7.Command != "npx" {
		t.Errorf("context7 command: %s", ctx7.Command)
	}
	if !ctx7.Enabled {
		t.Error("context7 should be enabled by default")
	}

	mem, ok := servers["memory"]
	if !ok {
		t.Fatal("missing memory server")
	}
	if mem.Command != "npx" {
		t.Errorf("memory command: %s", mem.Command)
	}
}

func TestReadGlobalMCPServersFrom_MissingFile(t *testing.T) {
	servers := ReadGlobalMCPServersFrom("/nonexistent/path/settings.json")
	if servers != nil {
		t.Error("expected nil for missing file")
	}
}

func TestBuildMCPConfigJSON(t *testing.T) {
	servers := map[string]MCPServer{
		"test-server": {Command: "node", Args: []string{"server.js"}, Enabled: true},
	}

	path, err := BuildMCPConfigJSON(servers)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	srv, ok := parsed.MCPServers["test-server"]
	if !ok {
		t.Fatal("missing test-server in output")
	}
	if srv.Command != "node" {
		t.Errorf("command mismatch: %s", srv.Command)
	}
	if len(srv.Args) != 1 || srv.Args[0] != "server.js" {
		t.Errorf("args mismatch: %v", srv.Args)
	}
}

func TestBuildMCPConfigJSON_Cleanup(t *testing.T) {
	servers := map[string]MCPServer{
		"s": {Command: "echo", Args: nil, Enabled: true},
	}
	path, err := BuildMCPConfigJSON(servers)
	if err != nil {
		t.Fatal(err)
	}

	// File exists
	if _, err := os.Stat(path); err != nil {
		t.Error("temp file should exist")
	}

	// Cleanup
	os.Remove(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("temp file should be removed after cleanup")
	}
}

func TestFilterEnabledMCP(t *testing.T) {
	servers := map[string]MCPServer{
		"enabled1":  {Command: "a", Enabled: true},
		"disabled1": {Command: "b", Enabled: false},
		"enabled2":  {Command: "c", Enabled: true},
	}

	filtered := FilterEnabledMCP(servers)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(filtered))
	}
	if _, ok := filtered["enabled1"]; !ok {
		t.Error("missing enabled1")
	}
	if _, ok := filtered["enabled2"]; !ok {
		t.Error("missing enabled2")
	}
	if _, ok := filtered["disabled1"]; ok {
		t.Error("disabled1 should not be in filtered result")
	}
}

func TestInitMCPFromGlobal_NilDoesNotPanic(t *testing.T) {
	cfg := DefaultConfig()
	// This calls ReadGlobalMCPServers internally which reads the actual global file.
	// We just verify it doesn't panic and sets MCPServers to non-nil if global exists.
	InitMCPFromGlobal(&cfg)
	// After init, MCPServers should be set (or remain nil if no global settings exist)
}

func TestInitMCPFromGlobal_AlreadySet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCPServers = map[string]MCPServer{
		"custom": {Command: "custom", Enabled: true},
	}
	InitMCPFromGlobal(&cfg)
	if len(cfg.MCPServers) != 1 {
		t.Error("InitMCPFromGlobal should not overwrite existing MCPServers")
	}
	if _, ok := cfg.MCPServers["custom"]; !ok {
		t.Error("custom server should still be present")
	}
}

func TestInstallMCPToGlobalAt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	// Write an initial settings file with one existing server
	initial := `{
		"mcpServers": {
			"existing": {
				"command": "npx",
				"args": ["-y", "@existing/mcp"]
			}
		},
		"permissions": {"allow": ["Read"]}
	}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Install a new server with env vars
	envVars := map[string]string{"API_KEY": "test123"}
	err := InstallMCPToGlobalAt(path, "new-server", "npx", []string{"-y", "@new/mcp"}, envVars)
	if err != nil {
		t.Fatalf("InstallMCPToGlobalAt failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON after install: %v", err)
	}

	// Verify other keys preserved
	if _, ok := raw["permissions"]; !ok {
		t.Error("permissions key should be preserved")
	}

	// Parse mcpServers
	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	var servers map[string]mcpEntry
	if err := json.Unmarshal(raw["mcpServers"], &servers); err != nil {
		t.Fatalf("parse mcpServers: %v", err)
	}

	// Existing server still present
	if _, ok := servers["existing"]; !ok {
		t.Error("existing server should still be present")
	}

	// New server added
	newSrv, ok := servers["new-server"]
	if !ok {
		t.Fatal("new-server should be added")
	}
	if newSrv.Command != "npx" {
		t.Errorf("command mismatch: %s", newSrv.Command)
	}
	if len(newSrv.Args) != 2 || newSrv.Args[1] != "@new/mcp" {
		t.Errorf("args mismatch: %v", newSrv.Args)
	}
	if newSrv.Env["API_KEY"] != "test123" {
		t.Errorf("env var mismatch: %v", newSrv.Env)
	}
}

func TestInstallMCPToGlobalAt_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new-settings.json")

	err := InstallMCPToGlobalAt(path, "first-server", "npx", []string{"-y", "@first/mcp"}, nil)
	if err != nil {
		t.Fatalf("InstallMCPToGlobalAt on new file failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	type mcpEntry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	var raw struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(raw.MCPServers))
	}
	srv, ok := raw.MCPServers["first-server"]
	if !ok {
		t.Fatal("first-server missing")
	}
	if srv.Command != "npx" {
		t.Errorf("command: %s", srv.Command)
	}
}

func TestDefaultSkeleton(t *testing.T) {
	sk := DefaultSkeleton()
	if !sk.WebSearch {
		t.Error("WebSearch should default to true")
	}
	if !sk.Context7Lookup {
		t.Error("Context7Lookup should default to true")
	}
	if !sk.BuildVerify {
		t.Error("BuildVerify should default to true")
	}
	if !sk.Test {
		t.Error("Test should default to true")
	}
	if !sk.PreCommit {
		t.Error("PreCommit should default to true")
	}
	if !sk.Commit {
		t.Error("Commit should default to true")
	}
	if sk.Push {
		t.Error("Push should default to false")
	}

	// Verify it's embedded in DefaultConfig
	cfg := DefaultConfig()
	if cfg.Skeleton.Push {
		t.Error("Config.Skeleton.Push should default to false")
	}
	if !cfg.Skeleton.WebSearch {
		t.Error("Config.Skeleton.WebSearch should default to true")
	}
}
