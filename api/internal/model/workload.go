package model

import "time"

// Workload status constants.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusKilled    = "killed"
)

// Isolation mode constants.
const (
	IsolationMicroVM = "microvm"
	IsolationIsolate = "isolate"
	IsolationGVisor  = "gvisor"
)

// Runtime constants.
const (
	RuntimeGo     = "go"
	RuntimeNode   = "node"
	RuntimePython = "python"
	RuntimeWasm   = "wasm"
	RuntimeOCI    = "oci"
)

// Workload represents a compute workload submitted to the platform.
type Workload struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Isolation  string     `json:"isolation"`
	Runtime    string     `json:"runtime"`
	NodeID     string     `json:"node_id"`
	InputHash  string     `json:"input_hash,omitempty"`
	Output     []byte     `json:"output,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Error      string     `json:"error,omitempty"`
	CPULimit   *int       `json:"cpu_limit,omitempty"`
	MemLimit   *int       `json:"mem_limit,omitempty"`
	TimeoutS   *int       `json:"timeout_s,omitempty"`
	DurationMS *int       `json:"duration_ms,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
