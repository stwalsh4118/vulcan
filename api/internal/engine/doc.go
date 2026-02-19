// Package engine provides the asynchronous workload execution engine.
// It orchestrates workload lifecycle by resolving backends via the registry,
// enforcing timeouts via context deadlines, and updating the store with
// execution results in real time.
package engine
