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

// UpdateWorkloadStatus updates the status of a workload. For terminal statuses
// (killed, completed, failed), it also sets finished_at.
func (s *SQLiteStore) UpdateWorkloadStatus(ctx context.Context, id, status string) error {
	var result sql.Result
	var err error

	if status == model.StatusKilled || status == model.StatusCompleted || status == model.StatusFailed {
		result, err = s.db.ExecContext(ctx,
			"UPDATE workloads SET status = ?, finished_at = ? WHERE id = ?",
			status, time.Now().UTC(), id,
		)
	} else {
		result, err = s.db.ExecContext(ctx,
			"UPDATE workloads SET status = ? WHERE id = ?",
			status, id,
		)
	}

	if err != nil {
		return fmt.Errorf("update workload status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
