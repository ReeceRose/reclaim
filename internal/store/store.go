package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// MediaStatusActive and MediaStatusMissing are the two lifecycle states of a
// media_files row. Active means the file exists on disk; missing means it was
// not seen on the last scan (soft-delete — row is kept for history).
const (
	MediaStatusActive  = "active"
	MediaStatusMissing = "missing"
)

// LibraryTypeMovies and LibraryTypeTV are the two values for media_files.library_type,
// determined at scan time by which root the file is under.
const (
	LibraryTypeMovies = "movies"
	LibraryTypeTV     = "tv"
)

// Store is the single top-level handle to the database.
// Callers access domain state through the typed sub-stores;
// the underlying connection pools are not exposed.
type Store struct {
	Media    *Media
	Jobs     *Jobs
	Profiles *Profiles
	Scans    *Scans
	Settings *Settings
	Stats    *Stats
	Events   *Events
	Metadata *Metadata

	w *sql.DB
	r *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := buildDSN(path)

	w, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	w.SetMaxOpenConns(1)

	r, err := sql.Open("sqlite", dsn)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	r.SetMaxOpenConns(25)

	s := &Store{
		w:        w,
		r:        r,
		Media:    &Media{r: r, w: w},
		Jobs:     &Jobs{r: r, w: w},
		Profiles: &Profiles{r: r, w: w},
		Scans:    &Scans{r: r, w: w},
		Settings: &Settings{r: r, w: w},
		Stats:    &Stats{r: r, w: w},
		Events:   &Events{r: r, w: w},
		Metadata: &Metadata{r: r, w: w},
	}

	if err := runMigrations(w); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.Settings.ensureSecret(context.Background()); err != nil {
		s.Close()
		return nil, fmt.Errorf("ensure session secret: %w", err)
	}
	if err := s.bootstrapIfNeeded(context.Background()); err != nil {
		s.Close()
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return errors.Join(s.w.Close(), s.r.Close())
}

// Version returns the highest applied migration version.
func (s *Store) Version() (int64, error) {
	return migrationVersion(s.r)
}

// CommitEncodeSwap atomically updates the media row after a verified filesystem
// swap, marks the job completed, and inserts a job_completed event. The swap
// must already have succeeded — a DB failure here leaves the file encoded on
// disk with the job still in verifying for reconcile to retry.
func (s *Store) CommitEncodeSwap(ctx context.Context, fileID, jobID, newSize int64, newFingerprint string, completedAt int64, message, meta string) (int64, error) {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := s.Media.ReplaceWithEncodedTx(ctx, tx, fileID, newSize, newFingerprint, completedAt); err != nil {
		return 0, err
	}
	if err := s.Jobs.MarkCompletedTx(ctx, tx, jobID, newSize, completedAt); err != nil {
		return 0, err
	}
	id, err := s.Events.InsertTx(ctx, tx, EventJobCompleted, SeverityInfo, message, meta)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// CompleteJob atomically marks a job completed and inserts a job_completed
// event in the same transaction. Returns the new event ID for broadcasting.
func (s *Store) CompleteJob(ctx context.Context, jobID, outputSize, completedAt int64, message, meta string) (int64, error) {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := s.Jobs.MarkCompletedTx(ctx, tx, jobID, outputSize, completedAt); err != nil {
		return 0, err
	}
	id, err := s.Events.InsertTx(ctx, tx, EventJobCompleted, SeverityInfo, message, meta)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// FailJob atomically marks a job failed and inserts a job_failed event.
// Returns the new event ID for broadcasting.
func (s *Store) FailJob(ctx context.Context, jobID int64, errMsg string, failedAt int64, meta string) (int64, error) {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := s.Jobs.MarkFailedTx(ctx, tx, jobID, errMsg, failedAt); err != nil {
		return 0, err
	}
	id, err := s.Events.InsertTx(ctx, tx, EventJobFailed, SeverityError, errMsg, meta)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// CancelJob atomically marks a job cancelled and inserts a job_cancelled event.
// Returns the new event ID for broadcasting.
func (s *Store) CancelJob(ctx context.Context, jobID, cancelledAt int64, meta string) (int64, error) {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := s.Jobs.MarkCancelledTx(ctx, tx, jobID, cancelledAt); err != nil {
		return 0, err
	}
	id, err := s.Events.InsertTx(ctx, tx, EventJobCancelled, SeverityInfo, "Job cancelled", meta)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// buildDSN embeds SQLite PRAGMAs as URI query parameters so they are applied
// to every connection the pool opens — not just the first.
// RawQuery is set directly (not via url.Values) to avoid percent-encoding the
// parentheses that the _pragma parameter syntax requires.
func buildDSN(path string) string {
	u := &url.URL{Scheme: "file", Path: path}
	u.RawQuery = "_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)"
	return u.String()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting a single
// scan function handle both single-row queries and iteration loops.
type rowScanner interface {
	Scan(dest ...any) error
}

// ctxRowQuerier is satisfied by both *sql.DB and *sql.Tx, letting helpers run a
// single-row query against either a pool or an open transaction.
type ctxRowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// btoi converts a bool to the 0/1 integer SQLite expects for boolean columns.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
