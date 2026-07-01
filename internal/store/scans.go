package store

import (
	"context"
	"database/sql"
)

type ScanRun struct {
	ID           int64
	Trigger      string
	StartedAt    int64
	CompletedAt  *int64
	FilesScanned int
	FilesAdded   int
	FilesUpdated int
	FilesMoved   int
	FilesRemoved int
	Errors       int
}

type Scans struct {
	r, w *sql.DB
}

func (s *Scans) Create(ctx context.Context, trigger string, startedAt int64) (int64, error) {
	res, err := s.w.ExecContext(ctx,
		"INSERT INTO scan_runs (trigger, started_at) VALUES (?, ?)", trigger, startedAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Complete updates the run record with final counters. run.ID must be set.
func (s *Scans) Complete(ctx context.Context, run *ScanRun) error {
	_, err := s.w.ExecContext(ctx, `
		UPDATE scan_runs SET
			completed_at = ?, files_scanned = ?, files_added = ?, files_updated = ?,
			files_moved = ?, files_removed = ?, errors = ?
		WHERE id = ?`,
		run.CompletedAt, run.FilesScanned, run.FilesAdded, run.FilesUpdated,
		run.FilesMoved, run.FilesRemoved, run.Errors, run.ID,
	)
	return err
}
