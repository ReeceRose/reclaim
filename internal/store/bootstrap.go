package store

import (
	"context"
	"fmt"

	"reclaim/internal/media"
)

// bootstrapIfNeeded repairs state on any boot where library_stats was never
// populated and predicted_savings_bytes were never computed. Fresh installs
// with no media are a no-op.
func (s *Store) bootstrapIfNeeded(ctx context.Context) error {
	if err := s.ensureJobEncodeSnapshot(ctx); err != nil {
		return fmt.Errorf("ensure job encode snapshot: %w", err)
	}
	if err := s.ensureMissingSince(ctx); err != nil {
		return fmt.Errorf("ensure missing_since: %w", err)
	}

	needsStats, err := s.Stats.needsRebuild(ctx)
	if err != nil {
		return fmt.Errorf("check stats bootstrap: %w", err)
	}
	needsSavings, err := s.Media.needsSavingsBackfill(ctx)
	if err != nil {
		return fmt.Errorf("check savings bootstrap: %w", err)
	}
	needsOversize, err := s.Media.needsOversizeBackfill(ctx)
	if err != nil {
		return fmt.Errorf("check oversize bootstrap: %w", err)
	}
	if !needsStats && !needsSavings && !needsOversize {
		return nil
	}

	if needsSavings {
		if _, err := s.Media.BackfillPredictedSavings(ctx); err != nil {
			return fmt.Errorf("backfill predicted savings: %w", err)
		}
	}
	if needsOversize {
		if _, err := s.Media.BackfillOversizeRatio(ctx); err != nil {
			return fmt.Errorf("backfill oversize ratio: %w", err)
		}
	}
	if needsStats {
		if err := s.Stats.Recompute(ctx); err != nil {
			return fmt.Errorf("rebuild library stats: %w", err)
		}
	}
	return nil
}

// ensureJobEncodeSnapshot repairs databases where migration 00009 was recorded
// but the snapshot columns are missing (can happen if goose advanced version
// tracking without applying every statement).
func (s *Store) ensureJobEncodeSnapshot(ctx context.Context) error {
	has, err := tableHasColumn(ctx, s.w, "transcode_jobs", "encode_preset")
	if err != nil {
		return err
	}
	if has {
		return nil
	}

	alters := []string{
		`ALTER TABLE transcode_jobs ADD COLUMN encode_preset TEXT`,
		`ALTER TABLE transcode_jobs ADD COLUMN encode_crf INTEGER`,
		`ALTER TABLE transcode_jobs ADD COLUMN encode_extra_args TEXT`,
	}
	for _, q := range alters {
		if _, err := s.w.ExecContext(ctx, q); err != nil {
			return err
		}
	}

	_, err = s.w.ExecContext(ctx, `
		UPDATE transcode_jobs
		SET encode_preset = (SELECT preset FROM transcode_profiles WHERE id = transcode_jobs.profile_id),
		    encode_crf = (SELECT crf FROM transcode_profiles WHERE id = transcode_jobs.profile_id),
		    encode_extra_args = (SELECT extra_args FROM transcode_profiles WHERE id = transcode_jobs.profile_id)
		WHERE encode_preset IS NULL`)
	return err
}

// ensureMissingSince repairs databases where migration 00012 was recorded but
// the column is absent. Every write path that soft-deletes a row references
// missing_since, so a missing column would break scanning outright.
func (s *Store) ensureMissingSince(ctx context.Context) error {
	has, err := tableHasColumn(ctx, s.w, "media_files", "missing_since")
	if err != nil {
		return err
	}
	if has {
		return nil
	}

	if _, err := s.w.ExecContext(ctx,
		`ALTER TABLE media_files ADD COLUMN missing_since INTEGER`,
	); err != nil {
		return err
	}
	_, err = s.w.ExecContext(ctx, `
		UPDATE media_files
		SET missing_since = COALESCE(last_probed_at, unixepoch())
		WHERE status = 'missing' AND missing_since IS NULL`)
	return err
}

// needsRebuild reports whether library_stats is empty or missing expected
// dimensions while active media exists.
func (s *Stats) needsRebuild(ctx context.Context) (bool, error) {
	var activeMedia int
	if err := s.r.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM media_files WHERE status = 'active'`,
	).Scan(&activeMedia); err != nil {
		return false, err
	}
	if activeMedia == 0 {
		return false, nil
	}

	var statRows int
	if err := s.r.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_stats`).Scan(&statRows); err != nil {
		return false, err
	}
	if statRows == 0 {
		return true, nil
	}

	var libRows int
	if err := s.r.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM library_stats WHERE dimension = ?`, dimLibrary,
	).Scan(&libRows); err != nil {
		return false, err
	}
	if libRows == 0 {
		return true, nil
	}

	// One-time upgrade path: rebuild installs that stored exact heights before
	// resolution stats were canonicalized into five stable buckets.
	var legacyResRows int
	if err := s.r.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM library_stats
		WHERE dimension = ?
		  AND bucket NOT IN (?, ?, ?, ?, ?, ?, ?)`,
		dimResolution, resBandUnknown, resBandSD, resBandHD, resBandFHD, resBandQHD, resBandUHD, resBand8K,
	).Scan(&legacyResRows); err != nil {
		return false, err
	}
	return legacyResRows > 0, nil
}

// needsSavingsBackfill reports whether probed non-HEVC rows still carry the
// default of zero predicted_savings_bytes.
func (m *Media) needsSavingsBackfill(ctx context.Context) (bool, error) {
	var n int
	err := m.r.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM media_files
		WHERE status = 'active'
		  AND probe_error IS NULL
		  AND video_codec IS NOT NULL
		  AND is_already_hevc = 0
		  AND size_bytes > 0
		  AND predicted_savings_bytes = 0`,
	).Scan(&n)
	return n > 0, err
}

// needsOversizeBackfill reports whether any probed row that has the inputs for
// an oversize ratio (size, duration, resolution) still carries the default 0.
func (m *Media) needsOversizeBackfill(ctx context.Context) (bool, error) {
	var n int
	err := m.r.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM media_files
		WHERE status = 'active'
		  AND probe_error IS NULL
		  AND size_bytes > 0
		  AND duration_seconds > 0
		  AND (COALESCE(width, 0) > 0 OR COALESCE(height, 0) > 0)
		  AND oversize_ratio = 0`,
	).Scan(&n)
	return n > 0, err
}

// BackfillOversizeRatio recomputes oversize_ratio from stored probe fields
// without re-running ffprobe. Returns the number of rows updated. Runs once on
// boot after the column is added; fresh scans set the ratio inline.
func (m *Media) BackfillOversizeRatio(ctx context.Context) (int, error) {
	rows, err := m.r.QueryContext(ctx, `
		SELECT id, video_codec, width, height, duration_seconds, size_bytes
		FROM media_files
		WHERE status = 'active'
		  AND probe_error IS NULL
		  AND size_bytes > 0
		  AND duration_seconds > 0
		  AND (COALESCE(width, 0) > 0 OR COALESCE(height, 0) > 0)
		  AND oversize_ratio = 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id       int64
		codec    *string
		width    *int
		height   *int
		duration *float64
		size     int64
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.codec, &r.width, &r.height, &r.duration, &r.size); err != nil {
			return 0, err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	updated := 0
	for _, r := range pending {
		ratio := media.OversizeRatio(r.codec, r.width, r.height, r.size, r.duration)
		if ratio <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE media_files SET oversize_ratio = ? WHERE id = ?`,
			ratio, r.id,
		); err != nil {
			return 0, err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

// BackfillPredictedSavings recomputes predicted_savings_bytes from stored probe
// fields without re-running ffprobe. Returns the number of rows updated.
func (m *Media) BackfillPredictedSavings(ctx context.Context) (int, error) {
	rows, err := m.r.QueryContext(ctx, `
		SELECT id, video_codec, is_already_hevc, size_bytes
		FROM media_files
		WHERE status = 'active'
		  AND probe_error IS NULL
		  AND video_codec IS NOT NULL
		  AND is_already_hevc = 0
		  AND size_bytes > 0
		  AND predicted_savings_bytes = 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id    int64
		codec string
		hevc  bool
		size  int64
	}
	var pending []row

	for rows.Next() {
		var r row
		var hevc int
		if err := rows.Scan(&r.id, &r.codec, &hevc, &r.size); err != nil {
			return 0, err
		}
		r.hevc = hevc != 0
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	tx, err := m.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	updated := 0
	for _, r := range pending {
		codec := r.codec
		savings := media.PredictedSavingsBytes(&codec, r.hevc, r.size)
		if _, err := tx.ExecContext(ctx,
			`UPDATE media_files SET predicted_savings_bytes = ? WHERE id = ?`,
			savings, r.id,
		); err != nil {
			return 0, err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}
