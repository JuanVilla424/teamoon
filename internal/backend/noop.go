package backend

import "context"

// NoopBackend is a test-only backend that returns a pre-configured result.
type NoopBackend struct {
	BackendName string
	Result      SpawnResult
}

func (b *NoopBackend) Name() string {
	if b.BackendName != "" {
		return b.BackendName
	}
	return "noop"
}

func (b *NoopBackend) Caps() Capabilities { return Capabilities{} }

func (b *NoopBackend) Available() error { return nil }

func (b *NoopBackend) Execute(_ context.Context, _ SpawnRequest, events chan<- Event) (SpawnResult, error) {
	close(events)
	return b.Result, nil
}

func (b *NoopBackend) ResolveModel(model, _ string) string { return model }
