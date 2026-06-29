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
	needsStats, err := s.Stats.needsRebuild(ctx)
	if err != nil {
		return fmt.Errorf("check stats bootstrap: %w", err)
	}
	needsSavings, err := s.Media.needsSavingsBackfill(ctx)
	if err != nil {
		return fmt.Errorf("check savings bootstrap: %w", err)
	}
	if !needsStats && !needsSavings {
		return nil
	}

	if needsSavings {
		if _, err := s.Media.BackfillPredictedSavings(ctx); err != nil {
			return fmt.Errorf("backfill predicted savings: %w", err)
		}
	}
	if needsStats {
		if err := s.Stats.Recompute(ctx); err != nil {
			return fmt.Errorf("rebuild library stats: %w", err)
		}
	}
	return nil
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

	// One-time upgrade path: pre-height-bucket installs stored sd/hd/uhd bands.
	var legacyResRows int
	if err := s.r.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM library_stats
		WHERE dimension = ? AND bucket IN ('sd', 'hd', 'uhd')`, dimResolution,
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
