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
	IsolationAuto    = "auto"
)

// Runtime constants.
const (
	RuntimeGo     = "go"
	RuntimeNode   = "node"
	RuntimePython = "python"
	RuntimeWasm   = "wasm"
	RuntimeOCI    = "oci"
)

// validTransitions maps each status to the set of statuses it may transition to.
var validTransitions = map[string]map[string]bool{
	StatusPending: {
		StatusRunning: true,
		StatusFailed:  true,
		StatusKilled:  true,
	},
	StatusRunning: {
		StatusCompleted: true,
		StatusFailed:    true,
		StatusKilled:    true,
	},
}

// ValidTransition reports whether transitioning from one status to another is allowed.
func ValidTransition(from, to string) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// LogLine represents a single persisted log line from a workload execution.
type LogLine struct {
	ID         int64     `json:"id"`
	WorkloadID string    `json:"workload_id"`
	Seq        int       `json:"seq"`
	Line       string    `json:"line"`
	CreatedAt  time.Time `json:"created_at"`
}

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
