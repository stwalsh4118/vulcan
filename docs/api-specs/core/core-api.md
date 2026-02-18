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
    Output     string     `json:"output"`
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
| Isolation | `microvm`, `isolate`, `gvisor` |
| Runtime | `go`, `node`, `python`, `wasm`, `oci` |

## Store Interface

```go
// internal/store/store.go
type Store interface {
    CreateWorkload(ctx context.Context, w *model.Workload) error
    GetWorkload(ctx context.Context, id string) (*model.Workload, error)
    ListWorkloads(ctx context.Context, limit, offset int) ([]*model.Workload, int, error)
    UpdateWorkloadStatus(ctx context.Context, id, status string) error
    Close() error
}

var ErrNotFound = errors.New("not found")
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

### GET /v1/workloads/:id

**Response:** `200 OK` — full Workload object.

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

**Errors:** `404` — workload not found.

### Error Format

All errors return:
```json
{"error": "description of the error"}
```

## Server (Go)

```go
// internal/api/server.go
func NewServer(addr string, store store.Store, logger *slog.Logger) *Server
func (s *Server) Run() error // blocks until SIGINT/SIGTERM, graceful shutdown
```

Middleware stack: RequestID, Recoverer, Logging, Metrics, CORS.

HTTP server timeouts: `ReadHeaderTimeout: 10s`, `WriteTimeout: 30s`.
