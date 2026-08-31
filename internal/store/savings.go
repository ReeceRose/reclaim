package store

import (
	"context"
	"database/sql"
)

// Savings is the typed sub-store for the append-only realized-savings ledger.
type Savings struct{ r, w *sql.DB }

// SavingsSummary is the instance-wide roll-up of every completed encode.
type SavingsSummary struct {
	FilesEncoded     int64
	OriginalBytes    int64
	OutputBytes      int64
	BytesSaved       int64
	EncodeSeconds    int64
	FirstCompletedAt *int64
	LastCompletedAt  *int64

	FilesEncoded7d  int64
	BytesSaved7d    int64
	FilesEncoded30d int64
	BytesSaved30d   int64

	PredictedSamples     int64
	PredictedSavingsSum  int64
	PredictedActualSum   int64
	DurationSamples      int64
	EstimatedDurationSum int64
	ActualDurationSum    int64

	BestSavedBytes int64
	BestPath       string
}

// SavingsBucket is one slice of the ledger grouped by codec, library, or resolution.
type SavingsBucket struct {
	Key           string
	FilesEncoded  int64
	OriginalBytes int64
	OutputBytes   int64
	BytesSaved    int64
}

// SavingsDay is one calendar day of realized savings, split by how the bytes
// were reclaimed. BytesReplaced is a net delta and may be negative — a
// redownload that upgrades to a larger release costs space rather than saving it.
type SavingsDay struct {
	Day           string
	FilesEncoded  int64
	BytesSaved    int64
	FilesReplaced int64
	BytesReplaced int64
}

// SavingsEntry is a single completed encode as recorded in the ledger.
type SavingsEntry struct {
	JobID         int64
	MediaFileID   int64
	Path          string
	LibraryType   string
	SourceCodec   *string
	Width         *int
	Height        *int
	OriginalBytes int64
	OutputBytes   int64
	BytesSaved    int64
	EncodeSeconds *int64
	CompletedAt   int64
}

const savingsResolutionCase = `
	CASE
	  WHEN COALESCE(width, 0) <= 0 AND COALESCE(height, 0) <= 0 THEN 'unknown'
	  WHEN COALESCE(width, 0) >= 7680 OR COALESCE(height, 0) >= 4320 THEN 'uhd8k'
	  WHEN COALESCE(width, 0) >= 3840 OR COALESCE(height, 0) >= 2160 THEN 'uhd'
	  WHEN COALESCE(width, 0) >= 2560 OR COALESCE(height, 0) >= 1440 THEN 'qhd'
	  WHEN COALESCE(width, 0) >= 1920 OR COALESCE(height, 0) >= 1080 THEN 'fhd'
	  WHEN COALESCE(width, 0) >= 1280 OR COALESCE(height, 0) >= 720 THEN 'hd'
	  ELSE 'sd'
	END`

// RecordTx appends a ledger row for a job about to be marked completed. It must
// run before the media row is rewritten to HEVC, since it captures the source
// codec and pre-encode dimensions that the swap destroys.
func (s *Savings) RecordTx(ctx context.Context, tx *sql.Tx, jobID, outputSize, completedAt int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO savings_ledger (
			job_id, media_file_id, path, library_type, source_codec,
			width, height, duration_seconds,
			original_size_bytes, output_size_bytes,
			predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
			profile_id, encode_preset, encode_crf, completed_at
		)
		SELECT j.id, j.media_file_id,
		       COALESCE(m.path, ''), COALESCE(m.library_type, 'unknown'),
		       LOWER(NULLIF(m.video_codec, '')),
		       m.width, m.height, m.duration_seconds,
		       j.original_size_bytes, ?,
		       j.predicted_savings_bytes, j.initial_estimated_duration_seconds,
		       CASE WHEN j.started_at IS NOT NULL AND ? > j.started_at
		            THEN ? - j.started_at END,
		       j.profile_id, COALESCE(j.encode_preset, p.preset), COALESCE(j.encode_crf, p.crf),
		       ?
		FROM transcode_jobs j
		LEFT JOIN media_files m ON m.id = j.media_file_id
		LEFT JOIN transcode_profiles p ON p.id = j.profile_id
		WHERE j.id = ?`,
		outputSize, completedAt, completedAt, completedAt, jobID,
	)
	return err
}

// Summary rolls the whole ledger up into the headline figures. now anchors the
// rolling 7/30-day windows.
func (s *Savings) Summary(ctx context.Context, now int64) (*SavingsSummary, error) {
	out := &SavingsSummary{}
	err := s.r.QueryRowContext(ctx, `
		SELECT
		  COUNT(*),
		  COALESCE(SUM(original_size_bytes), 0),
		  COALESCE(SUM(output_size_bytes), 0),
		  COALESCE(SUM(original_size_bytes - output_size_bytes), 0),
		  COALESCE(SUM(encode_seconds), 0),
		  MIN(completed_at),
		  MAX(completed_at),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN original_size_bytes - output_size_bytes ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN original_size_bytes - output_size_bytes ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN predicted_savings_bytes IS NOT NULL THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN predicted_savings_bytes IS NOT NULL THEN predicted_savings_bytes ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN predicted_savings_bytes IS NOT NULL THEN original_size_bytes - output_size_bytes ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN estimated_duration_seconds IS NOT NULL AND encode_seconds IS NOT NULL THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN estimated_duration_seconds IS NOT NULL AND encode_seconds IS NOT NULL THEN estimated_duration_seconds ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN estimated_duration_seconds IS NOT NULL AND encode_seconds IS NOT NULL THEN encode_seconds ELSE 0 END), 0)
		FROM savings_ledger WHERE source = 'encode'`,
		now-7*86400, now-7*86400, now-30*86400, now-30*86400,
	).Scan(
		&out.FilesEncoded, &out.OriginalBytes, &out.OutputBytes, &out.BytesSaved,
		&out.EncodeSeconds, &out.FirstCompletedAt, &out.LastCompletedAt,
		&out.FilesEncoded7d, &out.BytesSaved7d, &out.FilesEncoded30d, &out.BytesSaved30d,
		&out.PredictedSamples, &out.PredictedSavingsSum, &out.PredictedActualSum,
		&out.DurationSamples, &out.EstimatedDurationSum, &out.ActualDurationSum,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	err = s.r.QueryRowContext(ctx, `
		SELECT original_size_bytes - output_size_bytes, path
		FROM savings_ledger
		WHERE source = 'encode'
		ORDER BY original_size_bytes - output_size_bytes DESC, id
		LIMIT 1`,
	).Scan(&out.BestSavedBytes, &out.BestPath)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

// ByCodec groups realized savings by the pre-encode source codec, bucketing
// rows with no recorded codec under 'unknown' the way library_stats does.
// Those are history backfilled from job rows, whose source codec the encode
// swap had already overwritten, plus any file whose codec never probed. They
// are counted rather than dropped so the buckets sum to the lifetime total.
func (s *Savings) ByCodec(ctx context.Context) ([]SavingsBucket, error) {
	return s.buckets(ctx, `
		SELECT COALESCE(LOWER(NULLIF(source_codec, '')), 'unknown'), COUNT(*),
		       COALESCE(SUM(original_size_bytes), 0),
		       COALESCE(SUM(output_size_bytes), 0),
		       COALESCE(SUM(original_size_bytes - output_size_bytes), 0)
		FROM savings_ledger
		WHERE source = 'encode'
		GROUP BY 1
		ORDER BY 5 DESC, 1`)
}

// ByLibrary groups realized savings by library type.
func (s *Savings) ByLibrary(ctx context.Context) ([]SavingsBucket, error) {
	return s.buckets(ctx, `
		SELECT library_type, COUNT(*),
		       COALESCE(SUM(original_size_bytes), 0),
		       COALESCE(SUM(output_size_bytes), 0),
		       COALESCE(SUM(original_size_bytes - output_size_bytes), 0)
		FROM savings_ledger
		WHERE source = 'encode'
		GROUP BY library_type
		ORDER BY 5 DESC, 1`)
}

// ByResolution groups realized savings by the same resolution bands library_stats uses.
func (s *Savings) ByResolution(ctx context.Context) ([]SavingsBucket, error) {
	return s.buckets(ctx, `
		SELECT `+savingsResolutionCase+`, COUNT(*),
		       COALESCE(SUM(original_size_bytes), 0),
		       COALESCE(SUM(output_size_bytes), 0),
		       COALESCE(SUM(original_size_bytes - output_size_bytes), 0)
		FROM savings_ledger
		WHERE source = 'encode'
		GROUP BY 1
		ORDER BY 5 DESC, 1`)
}

func (s *Savings) buckets(ctx context.Context, q string) ([]SavingsBucket, error) {
	rows, err := s.r.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavingsBucket{}
	for rows.Next() {
		var b SavingsBucket
		if err := rows.Scan(&b.Key, &b.FilesEncoded, &b.OriginalBytes, &b.OutputBytes, &b.BytesSaved); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Daily returns per-day realized savings over the trailing window, oldest
// first. tzOffsetSeconds shifts the day boundary to the display timezone.
func (s *Savings) Daily(ctx context.Context, since int64, tzOffsetSeconds int) ([]SavingsDay, error) {
	rows, err := s.r.QueryContext(ctx, `
		SELECT date(completed_at + ?, 'unixepoch') AS day,
		       COALESCE(SUM(CASE WHEN source = 'encode' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN source = 'encode' THEN original_size_bytes - output_size_bytes ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN source = 'replace' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN source = 'replace' THEN original_size_bytes - output_size_bytes ELSE 0 END), 0)
		FROM savings_ledger
		WHERE completed_at >= ?
		GROUP BY day
		ORDER BY day`,
		tzOffsetSeconds, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavingsDay{}
	for rows.Next() {
		var d SavingsDay
		if err := rows.Scan(&d.Day, &d.FilesEncoded, &d.BytesSaved,
			&d.FilesReplaced, &d.BytesReplaced); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Recent returns the most recent ledger entries, newest first.
func (s *Savings) Recent(ctx context.Context, limit int) ([]SavingsEntry, error) {
	return s.entries(ctx, `ORDER BY completed_at DESC, id DESC`, limit)
}

// TopWins returns the single largest realized savings, biggest first.
func (s *Savings) TopWins(ctx context.Context, limit int) ([]SavingsEntry, error) {
	return s.entries(ctx, `ORDER BY original_size_bytes - output_size_bytes DESC, id`, limit)
}

func (s *Savings) entries(ctx context.Context, orderBy string, limit int) ([]SavingsEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.r.QueryContext(ctx, `
		SELECT job_id, media_file_id, path, library_type, source_codec,
		       width, height, original_size_bytes, output_size_bytes,
		       original_size_bytes - output_size_bytes, encode_seconds, completed_at
		FROM savings_ledger WHERE source = 'encode' `+orderBy+` LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavingsEntry{}
	for rows.Next() {
		var e SavingsEntry
		if err := rows.Scan(&e.JobID, &e.MediaFileID, &e.Path, &e.LibraryType, &e.SourceCodec,
			&e.Width, &e.Height, &e.OriginalBytes, &e.OutputBytes,
			&e.BytesSaved, &e.EncodeSeconds, &e.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReplacementSummary rolls up every recorded replacement — a file deleted and
// re-acquired out of band rather than re-encoded by the worker.
//
// BytesDelta is the net (original - output) and is signed: replacing a 1080p
// h264 release with a 4K HEVC one is a legitimate replacement that costs disk.
// Reclaimed and Added split that net into its two halves so the UI can show
// both instead of a single number that quietly nets a win against a cost.
type ReplacementSummary struct {
	Files          int64
	OriginalBytes  int64
	OutputBytes    int64
	BytesDelta     int64
	BytesReclaimed int64
	BytesAdded     int64
	Smaller        int64
	Larger         int64

	Files7d       int64
	BytesDelta7d  int64
	Files30d      int64
	BytesDelta30d int64

	FirstAt *int64
	LastAt  *int64

	BestSavedBytes int64
	BestPath       string
}

// ReplacementEntry is a single delete-and-reacquire pair as recorded in the ledger.
type ReplacementEntry struct {
	MediaFileID  int64
	Path         string
	PreviousPath string
	LibraryType  string
	MatchKind    string
	SourceCodec  *string
	ResultCodec  *string
	Width        *int
	Height       *int
	ResultWidth  *int
	ResultHeight *int
	OriginalSize int64
	OutputSize   int64
	BytesSaved   int64
	CompletedAt  int64
}

// Match kinds recorded on a replacement ledger row, naming which reconciliation
// found the pair.
const (
	// MatchKindSupersede is the same-name-new-container case: the file was
	// replaced in place and the scanner matched it by path stem.
	MatchKindSupersede = "supersede"
	// MatchKindRedownload is the delete-then-reacquire case: the old row had
	// already gone missing and a later arrival matched it on content identity.
	MatchKindRedownload = "redownload"
)

// recordReplacementTx appends a replacement row pairing a vanished media row
// with the file that replaced it. It runs inside the caller's transaction —
// always the same one that folds the two media rows together — so a ledger
// entry can never outlive the reconciliation that justified it, and must run
// before the old row is deleted.
//
// Rows with a non-positive size on either side are skipped: an unprobed or
// zero-byte row would contribute a meaningless delta to a lifetime total. So is
// a pair of equal size, which reclaims nothing and costs nothing: the fold is
// still worth performing — the two rows are one file's history — but a ledger
// of bytes has nothing to say about it, and an entry that moves no total only
// makes the replacement count disagree with the reclaimed and given-back
// figures it is meant to sum.
func recordReplacementTx(ctx context.Context, tx *sql.Tx, oldID, newID int64, matchKind string, at int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO savings_ledger (
			source, match_kind, media_file_id, path, previous_path, library_type,
			source_codec, result_codec, width, height, result_width, result_height,
			duration_seconds, original_size_bytes, output_size_bytes, completed_at
		)
		SELECT 'replace', ?, n.id, n.path, o.path,
		       COALESCE(NULLIF(n.library_type, ''), o.library_type),
		       LOWER(NULLIF(o.video_codec, '')), LOWER(NULLIF(n.video_codec, '')),
		       o.width, o.height, n.width, n.height,
		       o.duration_seconds, o.size_bytes, n.size_bytes, ?
		FROM media_files o
		JOIN media_files n ON n.id = ?
		WHERE o.id = ? AND o.size_bytes > 0 AND n.size_bytes > 0
		  AND o.size_bytes != n.size_bytes`,
		matchKind, at, newID, oldID,
	)
	return err
}

// RecordReplacementTx is the exported form of recordReplacementTx for callers
// outside this file that already hold a write transaction.
func (s *Savings) RecordReplacementTx(ctx context.Context, tx *sql.Tx, oldID, newID int64, matchKind string, at int64) error {
	return recordReplacementTx(ctx, tx, oldID, newID, matchKind, at)
}

// ReplacementSummary rolls the replacement half of the ledger up. now anchors
// the rolling 7/30-day windows, as in Summary.
func (s *Savings) ReplacementSummary(ctx context.Context, now int64) (*ReplacementSummary, error) {
	out := &ReplacementSummary{}
	const delta = "original_size_bytes - output_size_bytes"
	err := s.r.QueryRowContext(ctx, `
		SELECT
		  COUNT(*),
		  COALESCE(SUM(original_size_bytes), 0),
		  COALESCE(SUM(output_size_bytes), 0),
		  COALESCE(SUM(`+delta+`), 0),
		  COALESCE(SUM(CASE WHEN `+delta+` > 0 THEN `+delta+` ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+delta+` < 0 THEN -(`+delta+`) ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+delta+` > 0 THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN `+delta+` < 0 THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN `+delta+` ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN completed_at >= ? THEN `+delta+` ELSE 0 END), 0),
		  MIN(completed_at),
		  MAX(completed_at)
		FROM savings_ledger WHERE source = 'replace'`,
		now-7*86400, now-7*86400, now-30*86400, now-30*86400,
	).Scan(
		&out.Files, &out.OriginalBytes, &out.OutputBytes, &out.BytesDelta,
		&out.BytesReclaimed, &out.BytesAdded, &out.Smaller, &out.Larger,
		&out.Files7d, &out.BytesDelta7d, &out.Files30d, &out.BytesDelta30d,
		&out.FirstAt, &out.LastAt,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	err = s.r.QueryRowContext(ctx, `
		SELECT `+delta+`, previous_path
		FROM savings_ledger
		WHERE source = 'replace'
		ORDER BY `+delta+` DESC, id
		LIMIT 1`,
	).Scan(&out.BestSavedBytes, &out.BestPath)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

// ReplacementsByLibrary groups replacement deltas by library type.
func (s *Savings) ReplacementsByLibrary(ctx context.Context) ([]SavingsBucket, error) {
	return s.buckets(ctx, `
		SELECT library_type, COUNT(*),
		       COALESCE(SUM(original_size_bytes), 0),
		       COALESCE(SUM(output_size_bytes), 0),
		       COALESCE(SUM(original_size_bytes - output_size_bytes), 0)
		FROM savings_ledger
		WHERE source = 'replace'
		GROUP BY library_type
		ORDER BY 5 DESC, 1`)
}

// RecentReplacements returns the newest replacement pairs, newest first.
func (s *Savings) RecentReplacements(ctx context.Context, limit int) ([]ReplacementEntry, error) {
	return s.replacements(ctx, `ORDER BY completed_at DESC, id DESC`, limit)
}

// TopReplacements returns the replacements that reclaimed the most bytes,
// biggest first. Upgrades (a negative delta) sort to the bottom, so a library
// with nothing but upgrades returns them rather than an empty list.
func (s *Savings) TopReplacements(ctx context.Context, limit int) ([]ReplacementEntry, error) {
	return s.replacements(ctx, `ORDER BY original_size_bytes - output_size_bytes DESC, id`, limit)
}

func (s *Savings) replacements(ctx context.Context, orderBy string, limit int) ([]ReplacementEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.r.QueryContext(ctx, `
		SELECT media_file_id, path, COALESCE(previous_path, ''), library_type,
		       COALESCE(match_kind, ''), source_codec, result_codec,
		       width, height, result_width, result_height,
		       original_size_bytes, output_size_bytes,
		       original_size_bytes - output_size_bytes, completed_at
		FROM savings_ledger WHERE source = 'replace' `+orderBy+` LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReplacementEntry{}
	for rows.Next() {
		var e ReplacementEntry
		if err := rows.Scan(&e.MediaFileID, &e.Path, &e.PreviousPath, &e.LibraryType,
			&e.MatchKind, &e.SourceCodec, &e.ResultCodec,
			&e.Width, &e.Height, &e.ResultWidth, &e.ResultHeight,
			&e.OriginalSize, &e.OutputSize, &e.BytesSaved, &e.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
