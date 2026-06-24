package store

import (
	"context"
	"database/sql"
)

// Resolution band thresholds (by height in pixels). These MUST stay in sync
// with the CASE expression in Stats.Recompute below — the incremental path
// (resolutionBand) and the recompute path derive the same buckets, and a test
// asserts incremental == recompute to catch any drift.
const (
	resHeightSD = 720  // height < 720 → sd
	resHeightHD = 2160 // 720 <= height < 2160 → hd; >= 2160 → uhd
)

// CodecStat is the per-codec aggregate slice of the library.
type CodecStat struct {
	Codec                 string
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// ResolutionStat is the per-resolution-band aggregate slice of the library.
type ResolutionStat struct {
	Band                  string
	FileCount             int64
	TotalBytes            int64
	PredictedSavingsBytes int64
}

// LibraryStats is the precomputed overview the dashboard renders (§11, §12).
type LibraryStats struct {
	TotalFiles            int64
	TotalBytes            int64
	TotalRecoverableBytes int64
	ByCodec               []CodecStat
	ByResolution          []ResolutionStat
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
		FROM library_stats WHERE dimension = 'total' AND bucket = ''`,
	).Scan(&out.TotalFiles, &out.TotalBytes, &out.TotalRecoverableBytes)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	codecRows, err := s.r.QueryContext(ctx, `
		SELECT bucket, file_count, total_bytes, predicted_savings_bytes
		FROM library_stats WHERE dimension = 'codec'
		ORDER BY total_bytes DESC, bucket`)
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
		FROM library_stats WHERE dimension = 'resolution'
		ORDER BY total_bytes DESC, bucket`)
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

	return out, nil
}

// Recompute rebuilds library_stats from media_files in a single transaction.
// This is the source of truth / repair tool: incremental deltas are fast but
// drift-prone, so this exists to reconcile them (wired into force rescans and,
// per P9, the scheduled drift guard).
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
		          WHEN height IS NULL OR height <= 0 THEN 'unknown'
		          WHEN height < 720 THEN 'sd'
		          WHEN height < 2160 THEN 'hd'
		          ELSE 'uhd'
		        END,
		        COUNT(*), COALESCE(SUM(size_bytes), 0), COALESCE(SUM(predicted_savings_bytes), 0)
		 FROM media_files WHERE status = 'active'
		 GROUP BY 2`,
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
	if f.Status != "active" {
		return nil
	}
	codec := "unknown"
	if f.VideoCodec != nil && *f.VideoCodec != "" {
		codec = *f.VideoCodec
	}
	return []statBucket{
		{"total", ""},
		{"codec", codec},
		{"resolution", resolutionBand(f.Height)},
	}
}

// resolutionBand classifies a height into the same bands the Recompute CASE uses.
func resolutionBand(height *int) string {
	if height == nil || *height <= 0 {
		return "unknown"
	}
	switch {
	case *height < resHeightSD:
		return "sd"
	case *height < resHeightHD:
		return "hd"
	default:
		return "uhd"
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
