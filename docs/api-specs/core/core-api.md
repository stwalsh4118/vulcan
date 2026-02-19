# Core API Specification

## Server Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `VULCAN_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `VULCAN_DB_PATH` | `vulcan.db` | SQLite database path |
| `VULCAN_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

## Config (Go)

```go
// internal/config/config.go
type Config struct {
    ListenAddr string // from VULCAN_LISTEN_ADDR
    DBPath     string // from VULCAN_DB_PATH
    LogLevel   string // from VULCAN_LOG_LEVEL
}
func Load() Config
func NewLogger(w io.Writer, level string) *slog.Logger
```

## Workload Model

```go
// internal/model/workload.go
type Workload struct {
    ID         string     `json:"id"`
    Status     string     `json:"status"`
    Isolation  string     `json:"isolation"`
    Runtime    string     `json:"runtime"`
    NodeID     string     `json:"node_id"`
    InputHash  string     `json:"input_hash"`
    Output     []byte     `json:"output"`
    ExitCode   *int       `json:"exit_code"`
    Error      string     `json:"error"`
    CPULimit   *int       `json:"cpu_limit"`
    MemLimit   *int       `json:"mem_limit"`
    TimeoutS   *int       `json:"timeout_s"`
    DurationMS *int       `json:"duration_ms"`
    CreatedAt  time.Time  `json:"created_at"`
    StartedAt  *time.Time `json:"started_at"`
    FinishedAt *time.Time `json:"finished_at"`
}

func NewID() string // Returns 26-char ULID
```

### Constants

| Category | Values |
|----------|--------|
| Status | `pending`, `running`, `completed`, `failed`, `killed` |
| Isolation | `microvm`, `isolate`, `gvisor`, `auto` |
| Runtime | `go`, `node`, `python`, `wasm`, `oci` |

### State Transitions

```go
// internal/model/workload.go
func ValidTransition(from, to string) bool

// Allowed transitions:
// pending  → running, failed, killed
// running  → completed, failed, killed
// All others are invalid.
```

## Backend Interface

```go
// internal/backend/backend.go
type Backend interface {
    Execute(ctx context.Context, spec WorkloadSpec) (WorkloadResult, error)
    Capabilities() BackendCapabilities
    Cleanup(ctx context.Context, workloadID string) error
}

type WorkloadSpec struct {
    ID         string
    Runtime    string
    Isolation  string
    Code       string
    Input      []byte
    CPULimit   int
    MemLimitMB int
    TimeoutS   int
    LogWriter  func(line string) `json:"-"` // optional log callback
}

type WorkloadResult struct {
    ExitCode   int
    Output     []byte
    Error      string
    DurationMS int
    LogLines   []string
}

type BackendCapabilities struct {
    Name                string
    SupportedRuntimes   []string
    SupportedIsolations []string
    MaxConcurrency      int
}
```

## Backend Registry

```go
// internal/backend/registry.go
type Registry struct { /* ... */ }

func NewRegistry() *Registry
func (r *Registry) Register(isolation string, b Backend)
func (r *Registry) Resolve(isolation, runtime string) (Backend, error)
func (r *Registry) List() []BackendInfo

type BackendInfo struct {
    Name         string              `json:"name"`
    Capabilities BackendCapabilities `json:"capabilities"`
}
```

### Auto-Routing Rules

| Runtime | Resolved Isolation |
|---------|--------------------|
| `node`  | `isolate` |
| `wasm`  | `isolate` |
| `python`| `microvm` |
| `go`    | `microvm` |
| `oci`   | `gvisor` |

## Execution Engine

```go
// internal/engine/engine.go
const DefaultTimeoutS = 30

type Engine struct { /* store, registry, logger, wg, broker */ }

func NewEngine(s store.Store, reg *backend.Registry, logger *slog.Logger) *Engine
func (e *Engine) Submit(ctx context.Context, w *model.Workload) error
func (e *Engine) Wait()
func (e *Engine) Broker() *LogBroker
```

## Log Broker

```go
// internal/engine/logbroker.go
type LogBroker struct { /* ... */ }

func NewLogBroker() *LogBroker
func (b *LogBroker) Subscribe(workloadID string) (<-chan string, func())
func (b *LogBroker) Publish(workloadID string, line string)
func (b *LogBroker) Close(workloadID string)
```

## Store Interface

```go
// internal/store/store.go
type Store interface {
    CreateWorkload(ctx context.Context, w *model.Workload) error
    GetWorkload(ctx context.Context, id string) (*model.Workload, error)
    ListWorkloads(ctx context.Context, limit, offset int) ([]*model.Workload, int, error)
    UpdateWorkloadStatus(ctx context.Context, id, status string) error
    UpdateWorkload(ctx context.Context, w *model.Workload) error
    GetWorkloadStats(ctx context.Context) (*WorkloadStats, error)
    InsertLogLine(ctx context.Context, workloadID string, seq int, line string) error
    GetLogLines(ctx context.Context, workloadID string) ([]model.LogLine, error)
    Close() error
}

type WorkloadStats struct {
    Total            int            `json:"total"`
    CountByStatus    map[string]int `json:"count_by_status"`
    CountByIsolation map[string]int `json:"count_by_isolation"`
    AvgDurationMS    float64        `json:"avg_duration_ms"`
}

var ErrNotFound = errors.New("not found")
var ErrInvalidTransition = errors.New("invalid status transition")
```

SQLite implementation: `NewSQLiteStore(dbPath string) (*SQLiteStore, error)`

## REST Endpoints

### GET /healthz

**Response:** `200 OK`
```json
{"status": "ok"}
```

### GET /metrics

**Response:** `200 OK` — Prometheus text format

Registered metrics:
- `vulcan_http_requests_total{method, path, status}` (counter)
- `vulcan_http_request_duration_seconds{method, path}` (histogram)

### POST /v1/workloads

**Request:**
```json
{
  "runtime": "node",
  "isolation": "isolate",
  "code": "console.log('hello')",
  "input": {},
  "resources": {"cpus": 1, "mem_mb": 128, "timeout_s": 30}
}
```
- `runtime` is required; all other fields optional.

**Response:** `201 Created` — full Workload object with `status: "pending"`, generated ULID `id`.

**Errors:** `400` — missing runtime or invalid JSON.

### POST /v1/workloads/async

**Request:** Same body as `POST /v1/workloads`.

- `isolation` defaults to `auto` if omitted.
- Default timeout: 30s if not specified.

**Response:** `202 Accepted` — full Workload object with `status: "pending"`.

Execution happens asynchronously in a goroutine. Poll `GET /v1/workloads/:id` for status.

**Errors:** `400` — missing runtime or invalid JSON. `500` — engine submission failure.

### GET /v1/workloads/:id

**Response:** `200 OK` — full Workload object (reflects real-time status during execution).

**Errors:** `404` — workload not found.

### GET /v1/workloads

**Query params:** `limit` (default 20, max 100), `offset` (default 0).

**Response:** `200 OK`
```json
{
  "workloads": [...],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

### DELETE /v1/workloads/:id

Sets status to `killed`, sets `finished_at`.

**Response:** `200 OK` — updated Workload object.

**Errors:** `404` — workload not found. `409` — workload cannot be killed in its current state.

### GET /v1/workloads/:id/logs

Server-Sent Events stream of log lines from a running workload.

**Headers:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`

**Events:** Each log line sent as `data: <line>\n\n`. Multi-line data is split per SSE spec.

**Behavior:**
- If workload is in a terminal state, returns 200 with empty stream (closes immediately).
- Stream closes when workload finishes or client disconnects.

**Errors:** `404` — workload not found.

### GET /v1/workloads/:id/logs/history

Returns all persisted log lines for a workload as a JSON array.

**Response:** `200 OK`
```json
{
  "workload_id": "01ABCDEF...",
  "lines": [
    {"seq": 0, "line": "Starting execution...", "created_at": "2026-02-19T07:00:00Z"},
    {"seq": 1, "line": "Done.", "created_at": "2026-02-19T07:00:01Z"}
  ]
}
```

- `lines` is always an array (empty `[]` if no log lines exist, never `null`).
- Log lines are ordered by `seq` ascending.

**Errors:** `404` — workload not found.

### GET /v1/backends

**Response:** `200 OK`
```json
[
  {
    "name": "isolate",
    "capabilities": {
      "name": "v8-isolate",
      "supported_runtimes": ["node", "wasm"],
      "supported_isolations": ["isolate"],
      "max_concurrency": 10
    }
  }
]
```

### GET /v1/stats

**Response:** `200 OK`
```json
{
  "total": 42,
  "by_status": {"pending": 2, "running": 1, "completed": 35, "failed": 4},
  "by_isolation": {"microvm": 15, "isolate": 20, "gvisor": 7},
  "avg_duration_ms": 1250.5
}
```

### Error Format

All errors return:
```json
{"error": "description of the error"}
```

## Server (Go)

```go
// internal/api/server.go
func NewServer(addr string, s store.Store, reg *backend.Registry, eng *engine.Engine, logger *slog.Logger) *Server
func (s *Server) Run() error // blocks until SIGINT/SIGTERM, graceful shutdown
```

Middleware stack: RequestID, Recoverer, Logging, Metrics, CORS.

HTTP server timeouts: `ReadHeaderTimeout: 10s`, `WriteTimeout: 30s` (disabled for SSE endpoints).
