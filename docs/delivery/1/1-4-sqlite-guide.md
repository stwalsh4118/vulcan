# SQLite Package Guide

**Date**: 2026-02-18
**Package**: `modernc.org/sqlite`
**Docs**: https://pkg.go.dev/modernc.org/sqlite

## API Usage

### Driver Name

```go
import _ "modernc.org/sqlite"
```

Driver name: `"sqlite"`

### Opening a Database

```go
db, err := sql.Open("sqlite", "vulcan.db")   // file-based
db, err := sql.Open("sqlite", ":memory:")     // in-memory (for tests)
```

### WAL Mode

```go
_, err = db.Exec("PRAGMA journal_mode=WAL")
```

### URI Parameters

```go
db, err := sql.Open("sqlite", "file:vulcan.db?_pragma=foreign_keys(1)")
```

### Key Points

- Pure Go, no CGO required
- `database/sql` compatible — use standard `db.Query`, `db.Exec`, etc.
- Each connection is used by one goroutine at a time (database/sql handles pooling)
- Enable WAL mode for better concurrent read performance
