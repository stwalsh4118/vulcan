package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/seantiz/vulcan/internal/model"

	_ "modernc.org/sqlite"
)

const createWorkloadsTable = `
CREATE TABLE IF NOT EXISTS workloads (
    id          TEXT PRIMARY KEY,
    status      TEXT NOT NULL,
    isolation   TEXT NOT NULL,
    runtime     TEXT NOT NULL,
    node_id     TEXT NOT NULL,
    input_hash  TEXT,
    output      BLOB,
    exit_code   INTEGER,
    error       TEXT,
    cpu_limit   INTEGER,
    mem_limit   INTEGER,
    timeout_s   INTEGER,
    duration_ms INTEGER,
    created_at  DATETIME NOT NULL,
    started_at  DATETIME,
    finished_at DATETIME
)`

const createLogLinesTable = `
CREATE TABLE IF NOT EXISTS log_lines (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workload_id TEXT NOT NULL REFERENCES workloads(id),
    seq         INTEGER NOT NULL,
    line        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const createLogLinesIndex = `CREATE INDEX IF NOT EXISTS idx_log_lines_workload ON log_lines(workload_id, seq)`

// ErrNotFound is returned when a workload is not found.
var ErrNotFound = errors.New("workload not found")

// Compile-time interface satisfaction check.
var _ Store = (*SQLiteStore)(nil)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens the SQLite database at dbPath and runs migrations.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite does not benefit from multiple connections — writes are always
	// serialized. A single connection avoids "database is locked" errors and
	// ensures in-memory databases are consistent across goroutines.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.Exec(createWorkloadsTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create workloads table: %w", err)
	}

	if _, err := db.Exec(createLogLinesTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create log_lines table: %w", err)
	}

	if _, err := db.Exec(createLogLinesIndex); err != nil {
		db.Close()
		return nil, fmt.Errorf("create log_lines index: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// CreateWorkload inserts a new workload record.
func (s *SQLiteStore) CreateWorkload(ctx context.Context, w *model.Workload) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workloads (
			id, status, isolation, runtime, node_id, input_hash,
			output, exit_code, error, cpu_limit, mem_limit, timeout_s,
			duration_ms, created_at, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Status, w.Isolation, w.Runtime, w.NodeID, w.InputHash,
		w.Output, w.ExitCode, w.Error, w.CPULimit, w.MemLimit, w.TimeoutS,
		w.DurationMS, w.CreatedAt, w.StartedAt, w.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert workload: %w", err)
	}
	return nil
}

// GetWorkload retrieves a workload by ID.
func (s *SQLiteStore) GetWorkload(ctx context.Context, id string) (*model.Workload, error) {
	w := &model.Workload{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, status, isolation, runtime, node_id, input_hash,
			output, exit_code, error, cpu_limit, mem_limit, timeout_s,
			duration_ms, created_at, started_at, finished_at
		FROM workloads WHERE id = ?`, id,
	).Scan(
		&w.ID, &w.Status, &w.Isolation, &w.Runtime, &w.NodeID, &w.InputHash,
		&w.Output, &w.ExitCode, &w.Error, &w.CPULimit, &w.MemLimit, &w.TimeoutS,
		&w.DurationMS, &w.CreatedAt, &w.StartedAt, &w.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workload: %w", err)
	}
	return w, nil
}

// ListWorkloads returns a paginated list of workloads ordered by created_at DESC,
// along with the total count of all workloads.
func (s *SQLiteStore) ListWorkloads(ctx context.Context, limit, offset int) ([]*model.Workload, int, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, fmt.Errorf("begin read tx: %w", err)
	}
	defer tx.Rollback()

	var total int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM workloads").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count workloads: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, status, isolation, runtime, node_id, input_hash,
			output, exit_code, error, cpu_limit, mem_limit, timeout_s,
			duration_ms, created_at, started_at, finished_at
		FROM workloads ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list workloads: %w", err)
	}
	defer rows.Close()

	var workloads []*model.Workload
	for rows.Next() {
		w := &model.Workload{}
		if err := rows.Scan(
			&w.ID, &w.Status, &w.Isolation, &w.Runtime, &w.NodeID, &w.InputHash,
			&w.Output, &w.ExitCode, &w.Error, &w.CPULimit, &w.MemLimit, &w.TimeoutS,
			&w.DurationMS, &w.CreatedAt, &w.StartedAt, &w.FinishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan workload: %w", err)
		}
		workloads = append(workloads, w)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate workloads: %w", err)
	}

	return workloads, total, nil
}

// UpdateWorkloadStatus updates the status of a workload after validating the
// transition. For terminal statuses (killed, completed, failed), it also sets
// finished_at. For running, it sets started_at. Returns ErrInvalidTransition
// if the transition is not allowed, or ErrNotFound if the workload does not exist.
func (s *SQLiteStore) UpdateWorkloadStatus(ctx context.Context, id, status string) error {
	// SQLite serialises all writes via its file lock, so the read-then-update
	// pattern here is safe from TOCTOU races. Other store implementations (e.g.
	// Postgres) would need SELECT … FOR UPDATE or SERIALIZABLE isolation.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var current string
	err = tx.QueryRowContext(ctx, "SELECT status FROM workloads WHERE id = ?", id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read current status: %w", err)
	}

	if !model.ValidTransition(current, status) {
		return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, current, status)
	}

	now := time.Now().UTC()

	switch {
	case status == model.StatusRunning:
		_, err = tx.ExecContext(ctx,
			"UPDATE workloads SET status = ?, started_at = ? WHERE id = ?",
			status, now, id,
		)
	case status == model.StatusKilled || status == model.StatusCompleted || status == model.StatusFailed:
		_, err = tx.ExecContext(ctx,
			"UPDATE workloads SET status = ?, finished_at = ? WHERE id = ?",
			status, now, id,
		)
	default:
		_, err = tx.ExecContext(ctx,
			"UPDATE workloads SET status = ? WHERE id = ?",
			status, id,
		)
	}

	if err != nil {
		return fmt.Errorf("update workload status: %w", err)
	}

	return tx.Commit()
}

// UpdateWorkload updates the mutable fields of a workload: status, output,
// exit_code, error, duration_ms, started_at, and finished_at. Immutable fields
// (id, runtime, isolation, node_id, input_hash, cpu_limit, mem_limit, timeout_s,
// created_at) are not modified. Validates the state transition if the status has
// changed. Returns ErrNotFound if the workload does not exist, or
// ErrInvalidTransition if the status change is not allowed.
func (s *SQLiteStore) UpdateWorkload(ctx context.Context, w *model.Workload) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var current string
	err = tx.QueryRowContext(ctx, "SELECT status FROM workloads WHERE id = ?", w.ID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read current status: %w", err)
	}

	if current != w.Status && !model.ValidTransition(current, w.Status) {
		return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, current, w.Status)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE workloads SET
			status = ?, output = ?, exit_code = ?, error = ?,
			duration_ms = ?, started_at = ?, finished_at = ?
		WHERE id = ?`,
		w.Status, w.Output, w.ExitCode, w.Error,
		w.DurationMS, w.StartedAt, w.FinishedAt, w.ID,
	)
	if err != nil {
		return fmt.Errorf("update workload: %w", err)
	}

	return tx.Commit()
}

// InsertLogLine persists a single log line for a workload.
func (s *SQLiteStore) InsertLogLine(ctx context.Context, workloadID string, seq int, line string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO log_lines (workload_id, seq, line) VALUES (?, ?, ?)",
		workloadID, seq, line,
	)
	if err != nil {
		return fmt.Errorf("insert log line: %w", err)
	}
	return nil
}

// GetLogLines retrieves all log lines for a workload ordered by sequence number.
func (s *SQLiteStore) GetLogLines(ctx context.Context, workloadID string) ([]model.LogLine, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, workload_id, seq, line, created_at FROM log_lines WHERE workload_id = ? ORDER BY seq ASC",
		workloadID,
	)
	if err != nil {
		return nil, fmt.Errorf("query log lines: %w", err)
	}
	defer rows.Close()

	var lines []model.LogLine
	for rows.Next() {
		var l model.LogLine
		if err := rows.Scan(&l.ID, &l.WorkloadID, &l.Seq, &l.Line, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan log line: %w", err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate log lines: %w", err)
	}

	if lines == nil {
		lines = []model.LogLine{}
	}
	return lines, nil
}

// GetWorkloadStats returns aggregate execution statistics across all workloads.
// All queries run within a read-only transaction for a consistent snapshot.
func (s *SQLiteStore) GetWorkloadStats(ctx context.Context) (*WorkloadStats, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin read tx: %w", err)
	}
	defer tx.Rollback()

	stats := &WorkloadStats{
		CountByStatus:    make(map[string]int),
		CountByIsolation: make(map[string]int),
	}

	// Count by status.
	rows, err := tx.QueryContext(ctx, "SELECT status, COUNT(*) FROM workloads GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		stats.CountByStatus[status] = count
		stats.Total += count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status counts: %w", err)
	}

	// Count by isolation.
	rows2, err := tx.QueryContext(ctx,
		"SELECT isolation, COUNT(*) FROM workloads WHERE isolation != '' GROUP BY isolation")
	if err != nil {
		return nil, fmt.Errorf("count by isolation: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var isolation string
		var count int
		if err := rows2.Scan(&isolation, &count); err != nil {
			return nil, fmt.Errorf("scan isolation count: %w", err)
		}
		stats.CountByIsolation[isolation] = count
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("iterate isolation counts: %w", err)
	}

	// Average duration of completed workloads.
	var avgDuration sql.NullFloat64
	err = tx.QueryRowContext(ctx,
		"SELECT AVG(duration_ms) FROM workloads WHERE status = ? AND duration_ms IS NOT NULL",
		model.StatusCompleted,
	).Scan(&avgDuration)
	if err != nil {
		return nil, fmt.Errorf("avg duration: %w", err)
	}
	if avgDuration.Valid {
		stats.AvgDurationMS = avgDuration.Float64
	}

	return stats, nil
}
