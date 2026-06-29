package store

import (
	"context"
	"database/sql"
)

// Dimension names in library_stats.
const (
	dimTotal      = "total"
	dimCodec      = "codec"
	dimResolution = "resolution"
	dimLibrary    = "library"
)

// Resolution buckets used by library_stats and resolution filters, ordered
// low to high. Each class is "at least this standard" by width OR height.
const (
	resBandUnknown = "unknown"
	resBandSD      = "sd"
	resBandHD      = "hd"    // 720p
	resBandFHD     = "fhd"   // 1080p
	resBandQHD     = "qhd"   // 1440p
	resBandUHD     = "uhd"   // 4K/UHD
	resBand8K      = "uhd8k" // 8K
)

// CodecStat is the per-codec aggregate slice of the library.
type CodecStat struct {
	Codec                 string
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// ResolutionStat is the per-resolution-class aggregate slice of the library.
// Band holds "uhd8k", "uhd", "qhd", "fhd", "hd", "sd", or "unknown".
type ResolutionStat struct {
	Band                  string
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// LibraryStat is the per-library-type aggregate slice of the library.
type LibraryStat struct {
	LibraryType           string
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// LibraryStats is the precomputed overview the dashboard renders.
type LibraryStats struct {
	TotalFiles            int64
	TotalBytes            int64
	TotalRecoverableBytes int64
	ByCodec               []CodecStat
	ByResolution          []ResolutionStat
	ByLibrary             []LibraryStat
}

type Stats struct {
	r, w *sql.DB
}

// Overview reads the precomputed aggregates. It is O(buckets), not O(files):
// a handful of small reads regardless of library size.
func (s *Stats) Overview(ctx context.Context) (*LibraryStats, error) {
	out := &LibraryStats{}

	err := s.r.QueryRowContext(ctx, `
		SELECT file_count, total_bytes, predicted_savings_bytes
		FROM library_stats WHERE dimension = ? AND bucket = ''`,
		dimTotal,
	).Scan(&out.TotalFiles, &out.TotalBytes, &out.TotalRecoverableBytes)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	codecRows, err := s.r.QueryContext(ctx, `
		SELECT bucket, file_count, total_bytes, predicted_savings_bytes
		FROM library_stats WHERE dimension = ?
		ORDER BY total_bytes DESC, bucket`,
		dimCodec,
	)
	if err != nil {
		return nil, err
	}
	defer codecRows.Close()
	for codecRows.Next() {
		var c CodecStat
		if err := codecRows.Scan(&c.Codec, &c.FileCount, &c.TotalBytes, &c.PredictedSavingsBytes); err != nil {
			return nil, err
		}
		out.ByCodec = append(out.ByCodec, c)
	}
	if err := codecRows.Err(); err != nil {
		return nil, err
	}

	resRows, err := s.r.QueryContext(ctx, `
		SELECT bucket, file_count, total_bytes, predicted_savings_bytes
		FROM library_stats WHERE dimension = ?
		ORDER BY total_bytes DESC, bucket`,
		dimResolution,
	)
	if err != nil {
		return nil, err
	}
	defer resRows.Close()
	for resRows.Next() {
		var r ResolutionStat
		if err := resRows.Scan(&r.Band, &r.FileCount, &r.TotalBytes, &r.PredictedSavingsBytes); err != nil {
			return nil, err
		}
		out.ByResolution = append(out.ByResolution, r)
	}
	if err := resRows.Err(); err != nil {
		return nil, err
	}

	libRows, err := s.r.QueryContext(ctx, `
		SELECT bucket, file_count, total_bytes, predicted_savings_bytes
		FROM library_stats WHERE dimension = ?
		ORDER BY total_bytes DESC, bucket`,
		dimLibrary,
	)
	if err != nil {
		return nil, err
	}
	defer libRows.Close()
	for libRows.Next() {
		var l LibraryStat
		if err := libRows.Scan(&l.LibraryType, &l.FileCount, &l.TotalBytes, &l.PredictedSavingsBytes); err != nil {
			return nil, err
		}
		out.ByLibrary = append(out.ByLibrary, l)
	}
	if err := libRows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// Recompute rebuilds library_stats from media_files in a single transaction.
// This is the source of truth / repair tool: incremental deltas are fast but
// drift-prone, so this exists to reconcile them.
func (s *Stats) Recompute(ctx context.Context) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM library_stats"); err != nil {
		return err
	}

	stmts := []string{
		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 SELECT 'total', '', COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		 FROM media_files WHERE status = 'active'`,

		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 SELECT 'codec', COALESCE(NULLIF(video_codec, ''), 'unknown'),
		        COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		 FROM media_files WHERE status = 'active'
		 GROUP BY COALESCE(NULLIF(video_codec, ''), 'unknown')`,

		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 SELECT 'resolution',
		        CASE
		          WHEN COALESCE(width, 0) <= 0 AND COALESCE(height, 0) <= 0 THEN 'unknown'
		          WHEN COALESCE(width, 0) >= 7680 OR COALESCE(height, 0) >= 4320 THEN 'uhd8k'
		          WHEN COALESCE(width, 0) >= 3840 OR COALESCE(height, 0) >= 2160 THEN 'uhd'
		          WHEN COALESCE(width, 0) >= 2560 OR COALESCE(height, 0) >= 1440 THEN 'qhd'
		          WHEN COALESCE(width, 0) >= 1920 OR COALESCE(height, 0) >= 1080 THEN 'fhd'
		          WHEN COALESCE(width, 0) >= 1280 OR COALESCE(height, 0) >= 720 THEN 'hd'
		          ELSE 'sd'
		        END,
		        COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		 FROM media_files WHERE status = 'active'
		 GROUP BY 2`,

		`INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
		 SELECT 'library', library_type,
		        COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		 FROM media_files WHERE status = 'active'
		 GROUP BY library_type`,
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}

	// Drop empty buckets so the table matches the incremental path, which never
	// retains a zero-count bucket.
	if _, err := tx.ExecContext(ctx, "DELETE FROM library_stats WHERE file_count <= 0"); err != nil {
		return err
	}

	return tx.Commit()
}

// --- Incremental delta maintenance ----------------------------------------
// These operate on a *sql.Tx so the stat change commits atomically with the
// media_files write that triggered it. Called from the Media repo.

type statBucket struct {
	dimension string
	bucket    string
}

// contributionsFor returns the buckets a single active file adds to. Only
// active files contribute; a non-active file contributes nothing.
func contributionsFor(f *MediaFile) []statBucket {
	if f.Status != MediaStatusActive {
		return nil
	}
	codec := resBandUnknown
	if f.VideoCodec != nil && *f.VideoCodec != "" {
		codec = *f.VideoCodec
	}
	lib := resBandUnknown
	if f.LibraryType != "" {
		lib = f.LibraryType
	}
	return []statBucket{
		{dimTotal, ""},
		{dimCodec, codec},
		{dimResolution, resolutionBucket(f.Width, f.Height)},
		{dimLibrary, lib},
	}
}

// resolutionBucket maps probed dimensions to the same bucket key Recompute uses.
func resolutionBucket(width, height *int) string {
	w, h := 0, 0
	if width != nil {
		w = *width
	}
	if height != nil {
		h = *height
	}
	switch {
	case w <= 0 && h <= 0:
		return resBandUnknown
	case w >= 7680 || h >= 4320:
		return resBand8K
	case w >= 3840 || h >= 2160:
		return resBandUHD
	case w >= 2560 || h >= 1440:
		return resBandQHD
	case w >= 1920 || h >= 1080:
		return resBandFHD
	case w >= 1280 || h >= 720:
		return resBandHD
	default:
		return resBandSD
	}
}

// applyContribution adds (sign=+1) or removes (sign=-1) a file's contribution
// to every bucket it belongs to, within the given transaction. After a
// subtraction it deletes any bucket whose file_count fell to zero so the table
// stays equal to a from-scratch recompute.
func applyContribution(ctx context.Context, tx *sql.Tx, f *MediaFile, sign int64) error {
	buckets := contributionsFor(f)
	if buckets == nil {
		return nil
	}
	count := sign
	bytes := sign * f.SizeBytes
	savings := sign * f.PredictedSavingsBytes

	for _, b := range buckets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO library_stats (dimension, bucket, file_count, total_bytes, predicted_savings_bytes)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(dimension, bucket) DO UPDATE SET
				file_count = file_count + excluded.file_count,
				total_bytes = total_bytes + excluded.total_bytes,
				predicted_savings_bytes = predicted_savings_bytes + excluded.predicted_savings_bytes`,
			b.dimension, b.bucket, count, bytes, savings,
		); err != nil {
			return err
		}
		if sign < 0 {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM library_stats
				WHERE dimension = ? AND bucket = ? AND file_count <= 0`,
				b.dimension, b.bucket,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
