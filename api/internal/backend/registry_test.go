package backend_test

import (
	"context"
	"testing"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
)

// stubBackend is a minimal Backend for registry tests.
type stubBackend struct {
	name      string
	runtimes  []string
	isolation string
}

func (s *stubBackend) Execute(_ context.Context, _ backend.WorkloadSpec) (backend.WorkloadResult, error) {
	return backend.WorkloadResult{}, nil
}

func (s *stubBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                s.name,
		SupportedRuntimes:   s.runtimes,
		SupportedIsolations: []string{s.isolation},
		MaxConcurrency:      8,
	}
}

func (s *stubBackend) Cleanup(_ context.Context, _ string) error { return nil }

func TestRegistryRegisterAndList(t *testing.T) {
	reg := backend.NewRegistry()

	reg.Register(model.IsolationIsolate, &stubBackend{
		name: "isolate", runtimes: []string{model.RuntimeNode}, isolation: model.IsolationIsolate,
	})
	reg.Register(model.IsolationMicroVM, &stubBackend{
		name: "microvm", runtimes: []string{model.RuntimeGo}, isolation: model.IsolationMicroVM,
	})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d backends, want 2", len(list))
	}

	names := make(map[string]bool)
	for _, info := range list {
		names[info.Name] = true
	}
	if !names["isolate"] || !names["microvm"] {
		t.Errorf("expected isolate and microvm in list, got %v", names)
	}
}

func TestRegistryResolveExplicit(t *testing.T) {
	reg := backend.NewRegistry()
	isolateBackend := &stubBackend{name: "isolate", isolation: model.IsolationIsolate}
	reg.Register(model.IsolationIsolate, isolateBackend)

	b, err := reg.Resolve(model.IsolationIsolate, model.RuntimeNode)
	if err != nil {
		t.Fatalf("Resolve explicit: %v", err)
	}
	if b.Capabilities().Name != "isolate" {
		t.Errorf("resolved backend name = %q, want %q", b.Capabilities().Name, "isolate")
	}
}

func TestRegistryResolveExplicitNotRegistered(t *testing.T) {
	reg := backend.NewRegistry()

	_, err := reg.Resolve(model.IsolationGVisor, model.RuntimeOCI)
	if err == nil {
		t.Error("expected error for unregistered backend, got nil")
	}
}

func TestRegistryResolveAuto(t *testing.T) {
	reg := backend.NewRegistry()
	reg.Register(model.IsolationIsolate, &stubBackend{name: "isolate", isolation: model.IsolationIsolate})
	reg.Register(model.IsolationGVisor, &stubBackend{name: "gvisor", isolation: model.IsolationGVisor})
	reg.Register(model.IsolationMicroVM, &stubBackend{name: "microvm", isolation: model.IsolationMicroVM})

	tests := []struct {
		runtime      string
		expectedName string
	}{
		{model.RuntimeNode, "isolate"},
		{model.RuntimeOCI, "gvisor"},
		{model.RuntimeGo, "microvm"},
		{model.RuntimePython, "microvm"},
		{model.RuntimeWasm, "isolate"},
	}

	for _, tc := range tests {
		b, err := reg.Resolve(model.IsolationAuto, tc.runtime)
		if err != nil {
			t.Errorf("Resolve(auto, %s): %v", tc.runtime, err)
			continue
		}
		if b.Capabilities().Name != tc.expectedName {
			t.Errorf("Resolve(auto, %s) = %q, want %q", tc.runtime, b.Capabilities().Name, tc.expectedName)
		}
	}
}

func TestRegistryResolveAutoTargetNotRegistered(t *testing.T) {
	reg := backend.NewRegistry()
	// Register only isolate, not microvm or gvisor.
	reg.Register(model.IsolationIsolate, &stubBackend{name: "isolate", isolation: model.IsolationIsolate})

	// Go auto-routes to microvm, which is not registered.
	_, err := reg.Resolve(model.IsolationAuto, model.RuntimeGo)
	if err == nil {
		t.Error("expected error when auto-resolved backend not registered, got nil")
	}
}
