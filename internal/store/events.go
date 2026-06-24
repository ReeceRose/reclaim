package store

import (
	"context"
	"database/sql"
	"errors"
)

// Event type constants.
const (
	EventJobCompleted   = "job_completed"
	EventJobFailed      = "job_failed"
	EventJobCancelled   = "job_cancelled"
	EventScanCompleted  = "scan_completed"
	EventOrphanRestored = "orphan_restored"
)

// Event severity constants.
const (
	SeverityInfo  = "info"
	SeverityWarn  = "warn"
	SeverityError = "error"
)

// Event is one row in the events audit log.
type Event struct {
	ID        int64
	Type      string
	Severity  string
	Message   string
	Metadata  *string // nullable JSON blob
	CreatedAt int64
}

// EventFilter controls which events List returns.
type EventFilter struct {
	AfterID  int64  // keyset cursor: return events with id < AfterID (older)
	Limit    int    // default 50
	Severity string // optional; empty = all
	Type     string // optional; empty = all
}

// Events is the typed sub-store for the audit events log.
type Events struct{ r, w *sql.DB }

const eventCols = `id, type, severity, message, metadata, created_at`

func scanEvent(s rowScanner) (*Event, error) {
	var e Event
	err := s.Scan(&e.ID, &e.Type, &e.Severity, &e.Message, &e.Metadata, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &e, err
}

// Insert writes an event directly via the write pool. Use this for post-commit
// logging (scan_completed, orphan_restored, reconcile paths).
func (ev *Events) Insert(ctx context.Context, eventType, severity, message, metadata string) (int64, error) {
	return insertEvent(ctx, ev.w, eventType, severity, message, metadata)
}

// InsertTx writes an event inside the caller's transaction. Use this for job
// state changes that must be atomic with the job row update.
func (ev *Events) InsertTx(ctx context.Context, tx *sql.Tx, eventType, severity, message, metadata string) (int64, error) {
	return insertEvent(ctx, tx, eventType, severity, message, metadata)
}

type execContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertEvent(ctx context.Context, db execContexter, eventType, severity, message, metadata string) (int64, error) {
	const q = `INSERT INTO events (type, severity, message, metadata, created_at)
	           VALUES (?, ?, ?, ?, unixepoch())`
	var meta any
	if metadata != "" {
		meta = metadata
	}
	res, err := db.ExecContext(ctx, q, eventType, severity, message, meta)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List returns events newest-first, keyset-paginated on id DESC.
// AfterID=0 means start from the newest event.
func (ev *Events) List(ctx context.Context, f EventFilter) ([]Event, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []any{}
	where := "1=1"

	if f.AfterID > 0 {
		where += " AND id < ?"
		args = append(args, f.AfterID)
	}
	if f.Severity != "" {
		where += " AND severity = ?"
		args = append(args, f.Severity)
	}
	if f.Type != "" {
		where += " AND type = ?"
		args = append(args, f.Type)
	}

	args = append(args, limit)
	q := `SELECT ` + eventCols + ` FROM events WHERE ` + where + ` ORDER BY id DESC LIMIT ?`

	rows, err := ev.r.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}
