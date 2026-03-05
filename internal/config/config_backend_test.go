package config

import "testing"

func TestBackendFor_Default(t *testing.T) {
	cfg := Config{}
	if b := BackendFor(cfg, "myproject"); b != "claude" {
		t.Errorf("expected claude, got %s", b)
	}
}

func TestBackendFor_SpawnBackend(t *testing.T) {
	cfg := Config{
		Spawn: SpawnConfig{Backend: "opencode"},
	}
	if b := BackendFor(cfg, "any"); b != "opencode" {
		t.Errorf("expected opencode, got %s", b)
	}
}

func TestBackendFor_ProjectOverride(t *testing.T) {
	cfg := Config{
		Spawn:           SpawnConfig{Backend: "claude"},
		ProjectBackends: map[string]string{"special": "aider"},
	}
	if b := BackendFor(cfg, "special"); b != "aider" {
		t.Errorf("expected aider, got %s", b)
	}
	// Other projects fall back to spawn backend
	if b := BackendFor(cfg, "other"); b != "claude" {
		t.Errorf("expected claude, got %s", b)
	}
}

func TestBackendFor_EmptyProjectMap(t *testing.T) {
	cfg := Config{
		ProjectBackends: map[string]string{},
		Spawn:           SpawnConfig{Backend: "goose"},
	}
	if b := BackendFor(cfg, "any"); b != "goose" {
		t.Errorf("expected goose, got %s", b)
	}
}
