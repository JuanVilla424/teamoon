package backend

import "testing"

func TestGet_Claude(t *testing.T) {
	b, ok := Get("claude")
	if !ok {
		t.Fatal("claude should be registered")
	}
	if b.Name() != "claude" {
		t.Errorf("expected claude, got %s", b.Name())
	}
}

func TestGet_Unknown(t *testing.T) {
	_, ok := Get("nonexistent")
	if ok {
		t.Error("nonexistent backend should not be found")
	}
}

func TestRegister_Custom(t *testing.T) {
	noop := &NoopBackend{BackendName: "test-custom"}
	Register("test-custom", noop)
	defer func() {
		mu.Lock()
		delete(registry, "test-custom")
		mu.Unlock()
	}()

	b, ok := Get("test-custom")
	if !ok {
		t.Fatal("test-custom should be registered")
	}
	if b.Name() != "test-custom" {
		t.Errorf("expected test-custom, got %s", b.Name())
	}
}

func TestDefault_ReturnsClaude(t *testing.T) {
	b := Default()
	if b == nil {
		t.Fatal("Default should not be nil")
	}
	if b.Name() != "claude" {
		t.Errorf("Default should be claude, got %s", b.Name())
	}
}

func TestResolve_Empty(t *testing.T) {
	b := Resolve("")
	if b.Name() != "claude" {
		t.Errorf("empty name should resolve to claude, got %s", b.Name())
	}
}

func TestResolve_Known(t *testing.T) {
	b := Resolve("claude")
	if b.Name() != "claude" {
		t.Errorf("expected claude, got %s", b.Name())
	}
}

func TestResolve_Unknown(t *testing.T) {
	b := Resolve("unknown-backend")
	if b.Name() != "claude" {
		t.Errorf("unknown should fall back to claude, got %s", b.Name())
	}
}

func TestGet_OpenCode(t *testing.T) {
	b, ok := Get("opencode")
	if !ok {
		t.Fatal("opencode should be registered")
	}
	if b.Name() != "opencode" {
		t.Errorf("expected opencode, got %s", b.Name())
	}
}

func TestResolve_OpenCode(t *testing.T) {
	b := Resolve("opencode")
	if b.Name() != "opencode" {
		t.Errorf("expected opencode, got %s", b.Name())
	}
}
