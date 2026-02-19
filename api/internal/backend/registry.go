package backend

import (
	"fmt"
	"sort"
	"sync"

	"github.com/seantiz/vulcan/internal/model"
)

// autoRouting maps runtimes to their default isolation mode for auto-resolution.
var autoRouting = map[string]string{
	model.RuntimeNode:   model.IsolationIsolate,
	model.RuntimePython: model.IsolationMicroVM,
	model.RuntimeGo:     model.IsolationMicroVM,
	model.RuntimeWasm:   model.IsolationIsolate,
	model.RuntimeOCI:    model.IsolationGVisor,
}

// BackendInfo pairs a backend name with its capabilities.
type BackendInfo struct {
	Name         string              `json:"name"`
	Capabilities BackendCapabilities `json:"capabilities"`
}

// Registry holds registered backends and resolves which one to use for a given
// workload based on isolation mode and runtime.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

// NewRegistry creates an empty backend registry.
func NewRegistry() *Registry {
	return &Registry{
		backends: make(map[string]Backend),
	}
}

// Register adds a backend to the registry under the given name.
func (r *Registry) Register(name string, b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[name] = b
}

// Resolve returns the backend to use for the given isolation and runtime.
// If isolation is "auto", it uses the autoRouting table to pick the default.
// Returns an error if the resolved backend is not registered.
func (r *Registry) Resolve(isolation, runtime string) (Backend, error) {
	target := isolation
	if target == model.IsolationAuto {
		resolved, ok := autoRouting[runtime]
		if !ok {
			return nil, fmt.Errorf("no auto-routing rule for runtime %q", runtime)
		}
		target = resolved
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[target]
	if !ok {
		return nil, fmt.Errorf("backend %q is not registered", target)
	}
	return b, nil
}

// List returns information about all registered backends, sorted by name
// for a stable API response.
func (r *Registry) List() []BackendInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]BackendInfo, 0, len(r.backends))
	for name, b := range r.backends {
		infos = append(infos, BackendInfo{
			Name:         name,
			Capabilities: b.Capabilities(),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}
