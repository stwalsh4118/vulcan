package backend_test

import (
	"context"
	"errors"
	"testing"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
)

// mockBackend is a minimal Backend implementation used to verify the interface
// is implementable and the domain types are usable.
type mockBackend struct {
	executeFn func(ctx context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error)
}

func (m *mockBackend) Execute(ctx context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, spec)
	}
	return backend.WorkloadResult{ExitCode: 0, Output: []byte("ok")}, nil
}

func (m *mockBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                "mock",
		SupportedRuntimes:   []string{model.RuntimeNode, model.RuntimePython},
		SupportedIsolations: []string{model.IsolationIsolate},
		MaxConcurrency:      4,
	}
}

func (m *mockBackend) Cleanup(_ context.Context, _ string) error {
	return nil
}

// Compile-time check that mockBackend satisfies the Backend interface.
var _ backend.Backend = (*mockBackend)(nil)

func TestBackendInterface_Implementable(t *testing.T) {
	var b backend.Backend = &mockBackend{}

	spec := backend.WorkloadSpec{
		ID:        "test-id",
		Runtime:   model.RuntimeNode,
		Isolation: model.IsolationIsolate,
		Code:      "console.log('hello')",
		TimeoutS:  30,
	}

	result, err := b.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if string(result.Output) != "ok" {
		t.Errorf("expected output %q, got %q", "ok", string(result.Output))
	}
}

func TestBackendCapabilities(t *testing.T) {
	b := &mockBackend{}
	caps := b.Capabilities()

	if caps.Name != "mock" {
		t.Errorf("expected name %q, got %q", "mock", caps.Name)
	}
	if len(caps.SupportedRuntimes) != 2 {
		t.Errorf("expected 2 supported runtimes, got %d", len(caps.SupportedRuntimes))
	}
	if caps.MaxConcurrency != 4 {
		t.Errorf("expected max concurrency 4, got %d", caps.MaxConcurrency)
	}
}

func TestExecuteErrorPath(t *testing.T) {
	expectedErr := errors.New("execution failed")
	b := &mockBackend{
		executeFn: func(_ context.Context, _ backend.WorkloadSpec) (backend.WorkloadResult, error) {
			return backend.WorkloadResult{}, expectedErr
		},
	}

	result, err := b.Execute(context.Background(), backend.WorkloadSpec{ID: "err-test"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected zero-valued result, got exit code %d", result.ExitCode)
	}
}

func TestCleanup(t *testing.T) {
	b := &mockBackend{}
	if err := b.Cleanup(context.Background(), "workload-123"); err != nil {
		t.Fatalf("Cleanup returned unexpected error: %v", err)
	}
}

func TestIsolationAutoConstant(t *testing.T) {
	if model.IsolationAuto != "auto" {
		t.Errorf("expected IsolationAuto to be %q, got %q", "auto", model.IsolationAuto)
	}
}
