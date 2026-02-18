# ULID Package Guide

**Date**: 2026-02-18
**Package**: `github.com/oklog/ulid/v2`
**Docs**: https://pkg.go.dev/github.com/oklog/ulid/v2

## API Usage

### Simple ID Generation (recommended)

```go
import "github.com/oklog/ulid/v2"

id := ulid.Make() // Thread-safe, monotonic entropy
fmt.Println(id.String()) // "01AN4Z07BY79KA1307SR9X4MV3"
```

`Make()` is safe for concurrent use and uses a process-global monotonic entropy source.

### String Format

- 26 characters, Crockford Base32
- Lexicographically sortable
- First 10 chars = timestamp (48-bit ms), last 16 chars = entropy (80-bit)

### Parsing

```go
id, err := ulid.Parse("01AN4Z07BY79KA1307SR9X4MV3")
```

### Key Properties

- Thread-safe via `Make()`
- Monotonic ordering within same millisecond
- URL-safe, no special characters
- Implements `sql.Scanner` and `driver.Valuer` for database use
