package backend

import "sync"

var (
	mu       sync.RWMutex
	registry = map[string]Backend{
		"claude":   &ClaudeBackend{},
		"opencode": &OpenCodeBackend{},
	}
)

// Get returns the backend registered under the given name.
func Get(name string) (Backend, bool) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := registry[name]
	return b, ok
}

// Register adds or replaces a backend in the registry.
func Register(name string, b Backend) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = b
}

// Default returns the default backend (Claude).
func Default() Backend {
	mu.RLock()
	defer mu.RUnlock()
	return registry["claude"]
}

// Resolve returns the backend for the given name, falling back to Default.
func Resolve(name string) Backend {
	if name == "" {
		return Default()
	}
	if b, ok := Get(name); ok {
		return b
	}
	return Default()
}
