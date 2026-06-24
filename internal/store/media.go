package store

import (
	"context"
	"database/sql"
	"errors"
)

type MediaFile struct {
	ID                    int64
	Path                  string
	LibraryType           string
	SizeBytes             int64
	Mtime                 int64
	Fingerprint           string
	VideoCodec            *string
	VideoCodecProfile     *string
	Width                 *int
	Height                *int
	DurationSeconds       *float64
	BitrateKbps           *int
	AudioCodec            *string
	AudioChannels         *int
	ContainerFormat       *string
	IsAlreadyHEVC         bool
	PredictedSavingsBytes int64
	LastProbedAt          *int64
	ProbeError            *string
	Status                string
}

type Media struct {
	r, w *sql.DB
}

func (m *Media) GetByID(ctx context.Context, id int64) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE id = ?", id))
}

func (m *Media) GetByPath(ctx context.Context, path string) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE path = ?", path))
}

func (m *Media) GetByFingerprint(ctx context.Context, fp string) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx, mediaQ+" WHERE fingerprint = ? AND status = 'active'", fp))
}

func (m *Media) Insert(ctx context.Context, f *MediaFile) (int64, error) {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO media_files (
			path, library_type, size_bytes, mtime, fingerprint,
			video_codec, video_codec_profile, width, height, duration_seconds,
			bitrate_kbps, audio_codec, audio_channels, container_format,
			is_already_hevc, predicted_savings_bytes, last_probed_at, probe_error, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Path, f.LibraryType, f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height, f.DurationSeconds,
		f.BitrateKbps, f.AudioCodec, f.AudioChannels, f.ContainerFormat,
		btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes, f.LastProbedAt, f.ProbeError, f.Status,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	f.ID = id
	if err := applyContribution(ctx, tx, f, +1); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (m *Media) UpdateProbe(ctx context.Context, f *MediaFile) error {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Subtract the row's current contribution before overwriting it, then add
	// the new one — the stat table tracks the row's state, not its history.
	old, err := loadStatRow(ctx, tx, f.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if old != nil {
		if err := applyContribution(ctx, tx, old, -1); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE media_files SET
			size_bytes = ?, mtime = ?, fingerprint = ?,
			video_codec = ?, video_codec_profile = ?, width = ?, height = ?,
			duration_seconds = ?, bitrate_kbps = ?, audio_codec = ?, audio_channels = ?,
			container_format = ?, is_already_hevc = ?, predicted_savings_bytes = ?,
			last_probed_at = ?, probe_error = ?, status = ?
		WHERE id = ?`,
		f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height,
		f.DurationSeconds, f.BitrateKbps, f.AudioCodec, f.AudioChannels,
		f.ContainerFormat, btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes,
		f.LastProbedAt, f.ProbeError, f.Status, f.ID,
	); err != nil {
		return err
	}

	if err := applyContribution(ctx, tx, f, +1); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Media) UpdatePath(ctx context.Context, id int64, newPath string) error {
	_, err := m.w.ExecContext(ctx,
		"UPDATE media_files SET path = ?, status = 'active' WHERE id = ?", newPath, id,
	)
	return err
}

func (m *Media) MarkMissing(ctx context.Context, id int64) error {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// A row going missing leaves the library totals (applyContribution is a
	// no-op if the row was already inactive, keeping this idempotent).
	old, err := loadStatRow(ctx, tx, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if old != nil {
		if err := applyContribution(ctx, tx, old, -1); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET status = 'missing' WHERE id = ?", id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ActivePaths returns a path→id map of all active media files. Used by the
// scanner to diff the filesystem against known state.
func (m *Media) ActivePaths(ctx context.Context) (map[string]int64, error) {
	rows, err := m.r.QueryContext(ctx, "SELECT id, path FROM media_files WHERE status = 'active'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		out[path] = id
	}
	return out, rows.Err()
}

// FileSummary is the minimal record the scanner loads at the start of each diff
// to compare against the filesystem without pulling full probe data.
type FileSummary struct {
	ID          int64
	SizeBytes   int64
	Mtime       int64
	Fingerprint string
}

// ActiveFileSummaries returns a path→FileSummary map for all active files.
func (m *Media) ActiveFileSummaries(ctx context.Context) (map[string]*FileSummary, error) {
	rows, err := m.r.QueryContext(ctx,
		"SELECT id, path, size_bytes, mtime, fingerprint FROM media_files WHERE status = 'active'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]*FileSummary)
	for rows.Next() {
		var path string
		var s FileSummary
		if err := rows.Scan(&s.ID, &path, &s.SizeBytes, &s.Mtime, &s.Fingerprint); err != nil {
			return nil, err
		}
		out[path] = &s
	}
	return out, rows.Err()
}

// GetByFingerprintOtherThan returns the first active file with fp whose ID is
// not excludeID. Used by rename detection to find a newly-inserted row that
// matches a vanished file without returning the vanished row itself.
func (m *Media) GetByFingerprintOtherThan(ctx context.Context, fp string, excludeID int64) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx,
		mediaQ+" WHERE fingerprint = ? AND status = 'active' AND id != ?", fp, excludeID,
	))
}

// RecordMove updates keepID's path to newPath and deletes mergeID in a single
// transaction. Job history on keepID is preserved; mergeID is the duplicate row
// the scanner inserted for the renamed destination path.
func (m *Media) RecordMove(ctx context.Context, keepID, mergeID int64, newPath string) error {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// The merge row was just inserted for the destination path, so its
	// contribution is double-counting the same physical file as keepID. Remove
	// it before deleting the row.
	merge, err := loadStatRow(ctx, tx, mergeID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if merge != nil {
		if err := applyContribution(ctx, tx, merge, -1); err != nil {
			return err
		}
	}

	// DELETE the duplicate row first so the UNIQUE(path) constraint doesn't fire
	// when we update keepID's path to the same value.
	if _, err := tx.ExecContext(ctx, "DELETE FROM media_files WHERE id = ?", mergeID); err != nil {
		return err
	}

	// If keepID was inactive (e.g. previously marked missing), reactivating it
	// adds its contribution back. If it was already active it keeps counting.
	keep, err := loadStatRow(ctx, tx, keepID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if keep != nil && keep.Status != "active" {
		keep.Status = "active"
		if err := applyContribution(ctx, tx, keep, +1); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET path = ?, status = 'active' WHERE id = ?",
		newPath, keepID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceWithEncoded updates a media row after a verified HEVC swap: new size +
// fingerprint, video_codec forced to hevc, is_already_hevc set, and predicted
// savings zeroed (it's now HEVC — nothing left to reclaim). The library_stats
// deltas are applied in the same transaction so the dashboard reflects the
// reclaimed bytes immediately, and the is_already_hevc flip is what drops the
// file out of the candidate query, closing the loop.
func (m *Media) ReplaceWithEncoded(ctx context.Context, id, newSize int64, newFingerprint string, now int64) error {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	old, err := loadStatRow(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := applyContribution(ctx, tx, old, -1); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE media_files SET
			size_bytes = ?, fingerprint = ?, video_codec = 'hevc',
			is_already_hevc = 1, predicted_savings_bytes = 0,
			mtime = ?, last_probed_at = ?, probe_error = NULL
		WHERE id = ?`,
		newSize, newFingerprint, now, now, id,
	); err != nil {
		return err
	}

	updated, err := loadStatRow(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := applyContribution(ctx, tx, updated, +1); err != nil {
		return err
	}
	return tx.Commit()
}

// loadStatRow loads the stat-relevant fields of a row (used inside write
// transactions to compute deltas). q is *sql.Tx or *sql.DB.
func loadStatRow(ctx context.Context, q ctxRowQuerier, id int64) (*MediaFile, error) {
	f := &MediaFile{ID: id}
	err := q.QueryRowContext(ctx,
		"SELECT size_bytes, predicted_savings_bytes, video_codec, height, status FROM media_files WHERE id = ?",
		id,
	).Scan(&f.SizeBytes, &f.PredictedSavingsBytes, &f.VideoCodec, &f.Height, &f.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

const mediaQ = `
	SELECT id, path, library_type, size_bytes, mtime, fingerprint,
		video_codec, video_codec_profile, width, height, duration_seconds,
		bitrate_kbps, audio_codec, audio_channels, container_format,
		is_already_hevc, predicted_savings_bytes, last_probed_at, probe_error, status
	FROM media_files`

func scanMedia(s rowScanner) (*MediaFile, error) {
	var f MediaFile
	var isHEVC int
	err := s.Scan(
		&f.ID, &f.Path, &f.LibraryType, &f.SizeBytes, &f.Mtime, &f.Fingerprint,
		&f.VideoCodec, &f.VideoCodecProfile, &f.Width, &f.Height, &f.DurationSeconds,
		&f.BitrateKbps, &f.AudioCodec, &f.AudioChannels, &f.ContainerFormat,
		&isHEVC, &f.PredictedSavingsBytes, &f.LastProbedAt, &f.ProbeError, &f.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.IsAlreadyHEVC = isHEVC != 0
	return &f, nil
}
