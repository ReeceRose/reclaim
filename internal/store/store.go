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

// Store is the single top-level handle to the database.
// Callers access domain state through the typed sub-stores;
// the underlying connection pools are not exposed.
type Store struct {
	Media    *Media
	Jobs     *Jobs
	Profiles *Profiles
	Scans    *Scans
	Settings *Settings

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
	}

	if err := runMigrations(w); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.Settings.ensureSecret(context.Background()); err != nil {
		s.Close()
		return nil, fmt.Errorf("ensure session secret: %w", err)
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

// btoi converts a bool to the 0/1 integer SQLite expects for boolean columns.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
