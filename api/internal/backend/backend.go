package backend

import "context"

// Backend is the interface that all isolation backends must implement.
// Each backend (Firecracker microVM, V8 isolate, gVisor) provides its own
// implementation of these methods.
type Backend interface {
	// Execute runs a workload according to the given spec and returns the result.
	// The context carries deadlines and cancellation signals for timeout enforcement.
	Execute(ctx context.Context, spec WorkloadSpec) (WorkloadResult, error)

	// Capabilities reports what runtimes and isolation modes this backend supports.
	Capabilities() BackendCapabilities

	// Cleanup releases any resources associated with the given workload.
	Cleanup(ctx context.Context, workloadID string) error
}

// WorkloadSpec describes a workload to be executed by a backend.
type WorkloadSpec struct {
	ID         string `json:"id"`
	Runtime    string `json:"runtime"`
	Isolation  string `json:"isolation"`
	Code       string `json:"code"`
	Input      []byte `json:"input"`
	CPULimit   int    `json:"cpu_limit"`
	MemLimitMB int    `json:"mem_limit_mb"`
	TimeoutS   int    `json:"timeout_s"`

	// CodeArchive is a tar.gz archive containing the workload code.
	// When set, it takes precedence over the Code field.
	CodeArchive []byte `json:"code_archive,omitempty"`

	// LogWriter is an optional callback that backends invoke to emit log lines
	// during execution. Each call delivers one line to connected SSE subscribers.
	LogWriter func(line string) `json:"-"`
}

// WorkloadResult holds the output produced by a backend after executing a workload.
type WorkloadResult struct {
	ExitCode   int      `json:"exit_code"`
	Output     []byte   `json:"output"`
	Error      string   `json:"error"`
	DurationMS int      `json:"duration_ms"`
	LogLines   []string `json:"log_lines"`
}

// BackendCapabilities describes what a backend supports.
type BackendCapabilities struct {
	Name                string   `json:"name"`
	SupportedRuntimes   []string `json:"supported_runtimes"`
	SupportedIsolations []string `json:"supported_isolations"`
	MaxConcurrency      int      `json:"max_concurrency"`
}
