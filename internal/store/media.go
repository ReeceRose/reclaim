package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"reclaim/internal/media"
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
	OversizeRatio         float64
	LastProbedAt          *int64
	ProbeError            *string
	Status                string
	SeriesTitle           *string
	SeasonNumber          *int
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
			is_already_hevc, predicted_savings_bytes, oversize_ratio, last_probed_at, probe_error, status,
			series_title, season_number
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Path, f.LibraryType, f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height, f.DurationSeconds,
		f.BitrateKbps, f.AudioCodec, f.AudioChannels, f.ContainerFormat,
		btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes, f.OversizeRatio, f.LastProbedAt, f.ProbeError, f.Status,
		f.SeriesTitle, f.SeasonNumber,
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
			oversize_ratio = ?, last_probed_at = ?, probe_error = ?, status = ?,
			missing_since = CASE WHEN ? = 'missing' THEN COALESCE(missing_since, ?) END,
			series_title = ?, season_number = ?
		WHERE id = ?`,
		f.SizeBytes, f.Mtime, f.Fingerprint,
		f.VideoCodec, f.VideoCodecProfile, f.Width, f.Height,
		f.DurationSeconds, f.BitrateKbps, f.AudioCodec, f.AudioChannels,
		f.ContainerFormat, btoi(f.IsAlreadyHEVC), f.PredictedSavingsBytes,
		f.OversizeRatio, f.LastProbedAt, f.ProbeError, f.Status,
		f.Status, time.Now().Unix(),
		f.SeriesTitle, f.SeasonNumber, f.ID,
	); err != nil {
		return err
	}

	if err := applyContribution(ctx, tx, f, +1); err != nil {
		return err
	}
	return tx.Commit()
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

	// COALESCE keeps the original disappearance time: repeated marks (a watcher
	// event followed by a scan diff) must not restart the retention clock.
	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET status = 'missing', missing_since = COALESCE(missing_since, ?) WHERE id = ?",
		time.Now().Unix(), id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// MissingSummary describes the pool of missing rows a prune would consider.
type MissingSummary struct {
	Count       int   `json:"count"`
	OldestSince int64 `json:"oldest_since"`
	SizeBytes   int64 `json:"size_bytes"`
}

// MissingOverview counts rows currently soft-deleted, along with the oldest
// disappearance time and their combined last-known size. Powers the Settings
// panel's "N missing rows" line and the purge confirmation.
func (m *Media) MissingOverview(ctx context.Context) (*MissingSummary, error) {
	var out MissingSummary
	var oldest, size sql.NullInt64
	err := m.r.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(missing_since), SUM(size_bytes)
		FROM media_files WHERE status = 'missing'`,
	).Scan(&out.Count, &oldest, &size)
	if err != nil {
		return nil, err
	}
	out.OldestSince = oldest.Int64
	out.SizeBytes = size.Int64
	return &out, nil
}

// PruneMissing hard-deletes media rows that have been missing since at or
// before cutoff, along with their transcode_jobs history (the FK has no
// cascade, and the estimator's queries inner-join media_files anyway, so those
// job rows are already dead weight once the file row is gone). Rows with a
// live job — queued, running, or verifying — are left alone: the worker may
// still be mid-swap on a file that only looks absent.
//
// library_stats needs no adjustment: MarkMissing already removed each row's
// contribution when it went missing.
//
// Passing cutoff <= 0 deletes every missing row, which is what the manual
// purge does.
func (m *Media) PruneMissing(ctx context.Context, cutoff int64) (int64, error) {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Self-heal rows that are missing but carry no timestamp (a DB written before
	// the column existed, or one where the migration's backfill never ran). They
	// start their retention clock now rather than reading as infinitely old,
	// which would delete them on the very first prune.
	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET missing_since = ? WHERE status = 'missing' AND missing_since IS NULL",
		time.Now().Unix(),
	); err != nil {
		return 0, err
	}

	where := `
		status = 'missing'
		  AND NOT EXISTS (
		      SELECT 1 FROM transcode_jobs j
		      WHERE j.media_file_id = media_files.id
		        AND j.status IN ('queued', 'running', 'verifying')
		  )`
	var args []any
	if cutoff > 0 {
		where += " AND missing_since <= ?"
		args = append(args, cutoff)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM transcode_jobs
		WHERE media_file_id IN (SELECT id FROM media_files WHERE`+where+`)`,
		args...,
	); err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM media_files WHERE`+where, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
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

// ActivePaths returns filesystem paths for all active media rows.
func (m *Media) ActivePaths(ctx context.Context) ([]string, error) {
	rows, err := m.r.QueryContext(ctx,
		"SELECT path FROM media_files WHERE status = 'active'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// GetByFingerprintOtherThan returns the first active file with fp whose ID is
// not excludeID. Used by rename detection to find a newly-inserted row that
// matches a vanished file without returning the vanished row itself.
func (m *Media) GetByFingerprintOtherThan(ctx context.Context, fp string, excludeID int64) (*MediaFile, error) {
	return scanMedia(m.r.QueryRowContext(ctx,
		mediaQ+" WHERE fingerprint = ? AND status = 'active' AND id != ?", fp, excludeID,
	))
}

// FindSuperseder returns the active row that replaced a vanished file whose
// content changed as well as its path — an out-of-band re-encode that also
// changed container (`S07E01.mkv` → `S07E01.mp4`). Fingerprint matching cannot
// see these: the bytes differ by definition, so the name is the only signal
// left.
//
// The match is deliberately narrow — same directory, same name up to the final
// extension, and exactly one candidate. An ambiguous stem (several surviving
// siblings) returns ErrNotFound rather than a guess, and so does a stem that
// only matches by prefix (`Movie.mkv` must not claim `Movie.2160p.mkv`).
func (m *Media) FindSuperseder(ctx context.Context, oldPath string, excludeID int64) (*MediaFile, error) {
	dir, base := filepath.Dir(oldPath), filepath.Base(oldPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" || stem == base {
		return nil, ErrNotFound
	}

	// LIKE narrows to the same directory and stem prefix using the path index;
	// the exact stem equality is enforced in Go below.
	rows, err := m.r.QueryContext(ctx,
		mediaQ+` WHERE id != ? AND status = 'active' AND path LIKE ? ESCAPE '\'`,
		excludeID, likePrefix(filepath.Join(dir, stem)+"."),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var match *MediaFile
	for rows.Next() {
		f, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		if filepath.Dir(f.Path) != dir {
			continue
		}
		b := filepath.Base(f.Path)
		if !strings.EqualFold(strings.TrimSuffix(b, filepath.Ext(b)), stem) {
			continue
		}
		if match != nil {
			return nil, ErrNotFound
		}
		match = f
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if match == nil {
		return nil, ErrNotFound
	}
	return match, nil
}

// Supersede folds a vanished row into the row that replaced it: newID keeps its
// own probe data (it is a different encode, so oldID's codec and size are wrong
// for it), inherits oldID's job history, and oldID is hard-deleted.
//
// This is not RecordMove. A move keeps the old row and rewrites its path
// because the bytes are identical; here the bytes changed, so the surviving row
// has to be the freshly probed one.
//
// library_stats: applyContribution is a no-op on an inactive row, so a row
// already marked missing (which left the totals then) is unaffected, while one
// still active is discounted here before it goes.
//
// Returns ErrJobInFlight when oldID still has a queued, running, or verifying
// job — the worker may be mid-swap on a file that only looks absent.
func (m *Media) Supersede(ctx context.Context, oldID, newID int64) error {
	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var live int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM transcode_jobs
		WHERE media_file_id = ? AND status IN ('queued', 'running', 'verifying')`,
		oldID,
	).Scan(&live); err != nil {
		return err
	}
	if live > 0 {
		return ErrJobInFlight
	}

	old, err := loadStatRow(ctx, tx, oldID)
	if err != nil {
		return err
	}
	if err := applyContribution(ctx, tx, old, -1); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE transcode_jobs SET media_file_id = ? WHERE media_file_id = ?", newID, oldID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM media_files WHERE id = ?", oldID); err != nil {
		return err
	}
	return tx.Commit()
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
	if keep != nil && keep.Status != MediaStatusActive {
		keep.Status = MediaStatusActive
		if err := applyContribution(ctx, tx, keep, +1); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE media_files SET path = ?, status = 'active', missing_since = NULL WHERE id = ?",
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
	if err := m.ReplaceWithEncodedTx(ctx, tx, id, newSize, newFingerprint, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceWithEncodedTx is like ReplaceWithEncoded but runs inside the caller's
// transaction so it can be bundled with job completion in one commit.
func (m *Media) ReplaceWithEncodedTx(ctx context.Context, tx *sql.Tx, id, newSize int64, newFingerprint string, now int64) error {
	old, err := loadStatRow(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := applyContribution(ctx, tx, old, -1); err != nil {
		return err
	}

	// The file is now HEVC at a new (smaller) size, so its old oversize ratio —
	// computed from the pre-encode size and source codec — is stale. Recompute it
	// against the new size and hevc ceiling so the Library flag stays honest
	// without waiting for the next scan.
	var width, height *int
	var duration *float64
	if err := tx.QueryRowContext(ctx,
		"SELECT width, height, duration_seconds FROM media_files WHERE id = ?", id,
	).Scan(&width, &height, &duration); err != nil {
		return err
	}
	hevc := "hevc"
	oversize := media.OversizeRatio(&hevc, width, height, newSize, duration)

	if _, err := tx.ExecContext(ctx, `
		UPDATE media_files SET
			size_bytes = ?, fingerprint = ?, video_codec = 'hevc',
			is_already_hevc = 1, predicted_savings_bytes = 0, oversize_ratio = ?,
			mtime = ?, last_probed_at = ?, probe_error = NULL, status = ?,
			missing_since = NULL
		WHERE id = ?`,
		newSize, newFingerprint, oversize, now, now, MediaStatusActive, id,
	); err != nil {
		return err
	}

	updated, err := loadStatRow(ctx, tx, id)
	if err != nil {
		return err
	}
	return applyContribution(ctx, tx, updated, +1)
}

// UpdatePredictedSavingsByCodec rewrites predicted_savings_bytes for every
// active, non-HEVC file whose video_codec matches codec, using the supplied
// ratio (output/original). It returns the number of rows updated. The caller
// is responsible for calling Stats.Recompute after this to keep library_stats
// in sync, since this bypasses the per-row incremental delta path.
func (m *Media) UpdatePredictedSavingsByCodec(ctx context.Context, codec string, ratio float64) (int64, error) {
	res, err := m.w.ExecContext(ctx, `
		UPDATE media_files
		SET predicted_savings_bytes = CAST(size_bytes * ? AS INTEGER)
		WHERE status = 'active'
		  AND is_already_hevc = 0
		  AND LOWER(COALESCE(video_codec, '')) = ?`,
		1.0-ratio, codec,
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return n, err
}

// loadStatRow loads the stat-relevant fields of a row (used inside write
// transactions to compute deltas). q is *sql.Tx or *sql.DB.
func loadStatRow(ctx context.Context, q ctxRowQuerier, id int64) (*MediaFile, error) {
	f := &MediaFile{ID: id}
	err := q.QueryRowContext(ctx,
		"SELECT size_bytes, predicted_savings_bytes, video_codec, height, library_type, status FROM media_files WHERE id = ?",
		id,
	).Scan(&f.SizeBytes, &f.PredictedSavingsBytes, &f.VideoCodec, &f.Height, &f.LibraryType, &f.Status)
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
		is_already_hevc, predicted_savings_bytes, oversize_ratio, last_probed_at, probe_error, status,
		series_title, season_number
	FROM media_files`

func scanMedia(s rowScanner) (*MediaFile, error) {
	var f MediaFile
	var isHEVC int
	err := s.Scan(
		&f.ID, &f.Path, &f.LibraryType, &f.SizeBytes, &f.Mtime, &f.Fingerprint,
		&f.VideoCodec, &f.VideoCodecProfile, &f.Width, &f.Height, &f.DurationSeconds,
		&f.BitrateKbps, &f.AudioCodec, &f.AudioChannels, &f.ContainerFormat,
		&isHEVC, &f.PredictedSavingsBytes, &f.OversizeRatio, &f.LastProbedAt, &f.ProbeError, &f.Status,
		&f.SeriesTitle, &f.SeasonNumber,
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

// BackfillSeriesMeta populates series_title and season_number for all TV files
// that don't have them yet (rows added before the migration). Runs in a single
// transaction so the browse page becomes consistent immediately on startup.
func (m *Media) BackfillSeriesMeta(ctx context.Context, tvPath string) error {
	rows, err := m.r.QueryContext(ctx,
		"SELECT id, path FROM media_files WHERE library_type = 'tv' AND series_title IS NULL",
	)
	if err != nil {
		return err
	}
	type row struct {
		id   int64
		path string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.path); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"UPDATE media_files SET series_title = ?, season_number = ? WHERE id = ?",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range pending {
		title, season, _ := media.ParseTVInfo(r.path, tvPath)
		var titlePtr *string
		var seasonPtr *int
		if title != "" {
			titlePtr = &title
		}
		if season >= 0 {
			seasonPtr = &season
		}
		if _, err := stmt.ExecContext(ctx, titlePtr, seasonPtr, r.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (m *Media) DistinctSeriesTitles(ctx context.Context) ([]string, error) {
	rows, err := m.r.QueryContext(ctx,
		"SELECT DISTINCT series_title FROM media_files WHERE library_type='tv' AND status='active' AND series_title IS NOT NULL ORDER BY series_title",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		out = append(out, title)
	}
	return out, rows.Err()
}

func (m *Media) DistinctMovieKeys(ctx context.Context, moviesPath string) ([]string, error) {
	rows, err := m.r.QueryContext(ctx,
		"SELECT DISTINCT path FROM media_files WHERE library_type='movies' AND status='active' ORDER BY path",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	seen := make(map[string]bool)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		key := media.MovieKey(path, moviesPath)
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out, rows.Err()
}
