package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"reclaim/internal/jobs"
	"reclaim/internal/media"
)

type TranscodeJob struct {
	ID                 int64
	MediaFileID        int64
	ProfileID          int64
	Status             string
	QueuedAt           int64
	StartedAt          *int64
	CompletedAt        *int64
	OriginalSizeBytes  int64
	OutputSizeBytes    *int64
	ProgressPercent    float64
	OutputPath         *string
	ErrorMessage       *string
	VerificationResult *string
	Forced             bool
	// Snapshot of encode settings at queue time — used for learning, not for
	// the worker (which reads the live profile).
	EncodePreset    *string
	EncodeCRF       *int
	EncodeExtraArgs *string
	// PredictedSavingsBytes and InitialEstimatedDurationSeconds snapshot the
	// pre-encode predictions at queue time, so history can show how far off
	// the estimate was after the codec/size have since changed.
	PredictedSavingsBytes           *int64
	InitialEstimatedDurationSeconds *int64
	// SourcePath is the original media file path, populated only by the
	// path-joining list query (ListAllWithPath). It is nil elsewhere because
	// the file row may have been deleted after the job ran.
	SourcePath *string
	// Media probe fields populated only by ListWithPath / jobWithPathQ.
	DurationSeconds *float64
	Width           *int
	Height          *int
}

type Jobs struct {
	r, w *sql.DB
}

func (j *Jobs) Create(ctx context.Context, job *TranscodeJob) (int64, error) {
	res, err := j.w.ExecContext(ctx, `
		INSERT INTO transcode_jobs (
			media_file_id, profile_id, status, queued_at, original_size_bytes,
			encode_preset, encode_crf, encode_extra_args,
			predicted_savings_bytes, initial_estimated_duration_seconds
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.MediaFileID, job.ProfileID, job.Status, job.QueuedAt, job.OriginalSizeBytes,
		job.EncodePreset, job.EncodeCRF, job.EncodeExtraArgs,
		job.PredictedSavingsBytes, job.InitialEstimatedDurationSeconds,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (j *Jobs) GetByID(ctx context.Context, id int64) (*TranscodeJob, error) {
	return scanJob(j.r.QueryRowContext(ctx, jobQ+" WHERE id = ?", id))
}

// JobListQuery pages the combined queue + history list.
type JobListQuery struct {
	// Statuses, when non-empty, restricts results to jobs whose status is
	// any of these values (SQL IN).
	Statuses []string
	// OrderBy selects sort order: "" (default) orders oldest-first by
	// queued_at, matching queue position order; "recent" orders newest-first
	// by completion time (falling back to queued_at for jobs never started),
	// for the history view.
	OrderBy string
	Limit   int
	Offset  int
	// NoLimit skips the LIMIT/OFFSET clause entirely, returning every
	// matching row. Used for aggregate computations over the full queue
	// rather than a single page.
	NoLimit bool
}

const defaultJobLimit = 50
const maxJobLimit = 200

// ListWithPath returns one page of jobs, with SourcePath joined in.
func (j *Jobs) ListWithPath(ctx context.Context, q JobListQuery) ([]TranscodeJob, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultJobLimit
	}
	if limit > maxJobLimit {
		limit = maxJobLimit
	}

	query := jobWithPathQ + " WHERE j.dismissed_at IS NULL"
	var args []any
	if len(q.Statuses) > 0 {
		query += " AND j.status IN (" + placeholders(len(q.Statuses)) + ")"
		for _, s := range q.Statuses {
			args = append(args, s)
		}
	}
	if q.OrderBy == "recent" {
		query += " ORDER BY COALESCE(j.completed_at, j.queued_at) DESC, j.id DESC"
	} else {
		query += " ORDER BY j.queued_at ASC, j.id ASC"
	}
	if !q.NoLimit {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, q.Offset)
	}

	rows, err := j.r.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TranscodeJob
	for rows.Next() {
		job, err := scanJobWithPath(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// CountJobs returns how many non-dismissed jobs match an optional status filter.
func (j *Jobs) CountJobs(ctx context.Context, statuses []string) (int64, error) {
	query := `SELECT COUNT(*) FROM transcode_jobs WHERE dismissed_at IS NULL`
	var args []any
	if len(statuses) > 0 {
		query += " AND status IN (" + placeholders(len(statuses)) + ")"
		for _, s := range statuses {
			args = append(args, s)
		}
	}
	var n int64
	if err := j.r.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// QueuedPositions returns 1-based queue positions for every queued job.
func (j *Jobs) QueuedPositions(ctx context.Context) (map[int64]int, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT id FROM transcode_jobs
		WHERE status = 'queued'
		ORDER BY queued_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pos := make(map[int64]int)
	i := 1
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		pos[id] = i
		i++
	}
	return pos, rows.Err()
}

// ListAllWithPath returns every job, newest first, with the originating media
// file's path joined in (SourcePath). Used by GET /api/jobs so the UI can show
// the file name instead of a bare media_file_id. The LEFT JOIN keeps jobs whose
// media row was later deleted (SourcePath is nil in that case).
func (j *Jobs) ListAllWithPath(ctx context.Context) ([]TranscodeJob, error) {
	rows, err := j.r.QueryContext(ctx, jobWithPathQ+" ORDER BY j.queued_at DESC, j.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TranscodeJob
	for rows.Next() {
		job, err := scanJobWithPath(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, rows.Err()
}

// HasBlockingJob reports whether a media file already has a job that should keep
// it out of new queueing — anything queued/running/verifying/completed. A
// failed or cancelled job does not block a re-queue (mirrors jobExclusionSQL).
func (j *Jobs) HasBlockingJob(ctx context.Context, mediaFileID int64) (bool, error) {
	var n int
	err := j.r.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM transcode_jobs
		WHERE media_file_id = ?
		  AND status IN ('queued', 'running', 'verifying', 'completed')`,
		mediaFileID,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (j *Jobs) UpdateProgress(ctx context.Context, id int64, pct float64) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET progress_percent = ? WHERE id = ?", pct, id,
	)
	return err
}

// ClaimNextQueued atomically transitions the oldest queued job to running
// (stamping started_at) and returns it. The guarded UPDATE makes the claim safe
// even if a cancel raced in between the SELECT and the UPDATE: if the row is no
// longer queued the claim is abandoned. Returns ErrNotFound when the queue is
// empty.
func (j *Jobs) ClaimNextQueued(ctx context.Context, startedAt int64) (*TranscodeJob, error) {
	tx, err := j.w.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	job, err := scanJob(tx.QueryRowContext(ctx,
		jobQ+" WHERE status = 'queued' ORDER BY queued_at, id LIMIT 1"))
	if err != nil {
		return nil, err // ErrNotFound when empty
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE transcode_jobs SET status = 'running', started_at = ? WHERE id = ? AND status = 'queued'",
		startedAt, job.ID,
	)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound // lost the race; treat as empty
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	job.Status = string(jobs.StatusRunning)
	job.StartedAt = &startedAt
	return job, nil
}

// Transition performs a guarded status change. The move is first checked
// against the pure FSM in jobs.CanTransition, then applied with an UPDATE that
// only succeeds if the row is currently in `from` — serializing through the
// single writer and rejecting a state that changed underneath us. Returns
// ErrIllegalTransition for an FSM-illegal move or a stale source state.
func (j *Jobs) Transition(ctx context.Context, id int64, from, to string) error {
	if !jobs.CanTransition(jobs.Status(from), jobs.Status(to)) {
		return ErrIllegalTransition
	}
	res, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET status = ? WHERE id = ? AND status = ?", to, id, from,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIllegalTransition
	}
	return nil
}

// SetOutputPath records the temp output path the worker is encoding to, so a
// crash mid-encode leaves a breadcrumb the orphan sweep can reconcile.
func (j *Jobs) SetOutputPath(ctx context.Context, id int64, path string) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET output_path = ? WHERE id = ?", path, id,
	)
	return err
}

// ClearOutputPath removes the temp output breadcrumb once it's known no file
// exists at that path (e.g. an encode failure that never produced output).
func (j *Jobs) ClearOutputPath(ctx context.Context, id int64) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET output_path = NULL WHERE id = ?", id,
	)
	return err
}

// SetCommitError records a post-swap DB failure on a verifying job without
// changing its status, so reconcile can retry the commit on next boot.
func (j *Jobs) SetCommitError(ctx context.Context, id int64, msg string) error {
	_, err := j.w.ExecContext(ctx,
		`UPDATE transcode_jobs SET error_message = ? WHERE id = ? AND status = 'verifying'`,
		msg, id)
	return err
}

func (j *Jobs) SetVerificationResult(ctx context.Context, id int64, result string) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET verification_result = ? WHERE id = ?", result, id,
	)
	return err
}

// OutputPaths returns temp output paths recorded on non-terminal jobs for
// orphan sweep — avoids walking the full media tree.
func (j *Jobs) OutputPaths(ctx context.Context) ([]string, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT output_path FROM transcode_jobs
		WHERE output_path IS NOT NULL AND status IN ('queued', 'running', 'verifying')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// MarkCompleted moves a verifying job to completed, recording the output size
// and stamping 100% progress. Guarded on the verifying state.
func (j *Jobs) MarkCompleted(ctx context.Context, id, outputSize, completedAt int64) error {
	return j.terminal(ctx, id, "completed", []string{"verifying"},
		"output_size_bytes = ?, completed_at = ?, progress_percent = 100",
		outputSize, completedAt,
	)
}

// MarkFailed moves a running/verifying job to failed with an error message.
func (j *Jobs) MarkFailed(ctx context.Context, id int64, msg string, completedAt int64) error {
	return j.terminal(ctx, id, "failed", []string{"running", "verifying"},
		"error_message = ?, completed_at = ?",
		msg, completedAt,
	)
}

// MarkCancelled moves a queued/running/verifying job to cancelled.
func (j *Jobs) MarkCancelled(ctx context.Context, id, completedAt int64) error {
	return j.terminal(ctx, id, "cancelled", []string{"queued", "running", "verifying"},
		"completed_at = ?",
		completedAt,
	)
}

// MarkCompletedTx is like MarkCompleted but operates inside the caller's tx.
func (j *Jobs) MarkCompletedTx(ctx context.Context, tx *sql.Tx, id, outputSize, completedAt int64) error {
	return terminalTx(ctx, tx, id, "completed", []string{"verifying"},
		"output_size_bytes = ?, completed_at = ?, progress_percent = 100",
		outputSize, completedAt,
	)
}

// MarkFailedTx is like MarkFailed but operates inside the caller's tx.
func (j *Jobs) MarkFailedTx(ctx context.Context, tx *sql.Tx, id int64, msg string, completedAt int64) error {
	return terminalTx(ctx, tx, id, "failed", []string{"running", "verifying"},
		"error_message = ?, completed_at = ?",
		msg, completedAt,
	)
}

// MarkCancelledTx is like MarkCancelled but operates inside the caller's tx.
func (j *Jobs) MarkCancelledTx(ctx context.Context, tx *sql.Tx, id, completedAt int64) error {
	return terminalTx(ctx, tx, id, "cancelled", []string{"queued", "running", "verifying"},
		"completed_at = ?",
		completedAt,
	)
}

// terminal performs a guarded terminal-state UPDATE: status flips to `to` only
// if the row is currently in one of `from`, with extra column assignments
// applied in the same statement. Returns ErrIllegalTransition if no row matched.
func (j *Jobs) terminal(ctx context.Context, id int64, to string, from []string, setClause string, args ...any) error {
	placeholders := make([]string, len(from))
	sqlArgs := append([]any{}, args...)
	sqlArgs = append(sqlArgs, id)
	for i, s := range from {
		placeholders[i] = "?"
		sqlArgs = append(sqlArgs, s)
	}
	q := "UPDATE transcode_jobs SET status = '" + to + "', " + setClause +
		" WHERE id = ? AND status IN (" + strings.Join(placeholders, ",") + ")"
	res, err := j.w.ExecContext(ctx, q, sqlArgs...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIllegalTransition
	}
	return nil
}

// terminalTx is like terminal but executes inside the caller's transaction.
func terminalTx(ctx context.Context, tx *sql.Tx, id int64, to string, from []string, setClause string, args ...any) error {
	placeholders := make([]string, len(from))
	sqlArgs := append([]any{}, args...)
	sqlArgs = append(sqlArgs, id)
	for i, s := range from {
		placeholders[i] = "?"
		sqlArgs = append(sqlArgs, s)
	}
	q := "UPDATE transcode_jobs SET status = '" + to + "', " + setClause +
		" WHERE id = ? AND status IN (" + strings.Join(placeholders, ",") + ")"
	res, err := tx.ExecContext(ctx, q, sqlArgs...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIllegalTransition
	}
	return nil
}

// Dismiss hides a job from the history list without deleting its row, so it
// keeps contributing to LearnedRatios and the completed-job dedupe guard in
// jobExclusionSQL. Only terminal jobs (completed, failed, cancelled) can be
// dismissed. Returns ErrNotFound if no such row exists, or
// ErrIllegalTransition if the job is still queued/running/verifying.
func (j *Jobs) Dismiss(ctx context.Context, id, dismissedAt int64) error {
	res, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET dismissed_at = ? WHERE id = ? AND status IN ('completed', 'failed', 'cancelled')",
		dismissedAt, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var count int
		if serr := j.r.QueryRowContext(ctx, "SELECT COUNT(1) FROM transcode_jobs WHERE id = ?", id).Scan(&count); serr == nil && count == 0 {
			return ErrNotFound
		}
		return ErrIllegalTransition
	}
	return nil
}

// Force marks a queued job as forced, allowing the worker to run it outside the
// encode window. Returns ErrNotFound if the job does not exist, ErrIllegalTransition
// if it is not in the queued state.
func (j *Jobs) Force(ctx context.Context, id int64) error {
	res, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET forced = 1 WHERE id = ? AND status = 'queued'", id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// Distinguish "not found at all" from "wrong state".
		var count int
		if serr := j.r.QueryRowContext(ctx, "SELECT COUNT(1) FROM transcode_jobs WHERE id = ?", id).Scan(&count); serr == nil && count == 0 {
			return ErrNotFound
		}
		return ErrIllegalTransition
	}
	return nil
}

// ClaimNextForcedQueued is like ClaimNextQueued but only considers jobs with
// forced = 1. Used by the worker to drain forced jobs outside the encode window.
func (j *Jobs) ClaimNextForcedQueued(ctx context.Context, startedAt int64) (*TranscodeJob, error) {
	tx, err := j.w.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	job, err := scanJob(tx.QueryRowContext(ctx,
		jobQ+" WHERE status = 'queued' AND forced = 1 ORDER BY queued_at, id LIMIT 1"))
	if err != nil {
		return nil, err
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE transcode_jobs SET status = 'running', started_at = ? WHERE id = ? AND status = 'queued'",
		startedAt, job.ID,
	)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	job.Status = string(jobs.StatusRunning)
	job.StartedAt = &startedAt
	return job, nil
}

// ListInterrupted returns jobs left in running/verifying — i.e. jobs that were
// in flight when the process died. Crash recovery reconciles these.
func (j *Jobs) ListInterrupted(ctx context.Context) ([]TranscodeJob, error) {
	rows, err := j.r.QueryContext(ctx,
		jobQ+" WHERE status IN ('running', 'verifying') ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TranscodeJob
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, rows.Err()
}

const jobQ = `
	SELECT id, media_file_id, profile_id, status, queued_at, started_at, completed_at,
		original_size_bytes, output_size_bytes, progress_percent, output_path,
		error_message, verification_result, forced,
		encode_preset, encode_crf, encode_extra_args,
		predicted_savings_bytes, initial_estimated_duration_seconds
	FROM transcode_jobs`

func scanJob(s rowScanner) (*TranscodeJob, error) {
	var j TranscodeJob
	err := s.Scan(
		&j.ID, &j.MediaFileID, &j.ProfileID, &j.Status, &j.QueuedAt,
		&j.StartedAt, &j.CompletedAt, &j.OriginalSizeBytes, &j.OutputSizeBytes,
		&j.ProgressPercent, &j.OutputPath, &j.ErrorMessage, &j.VerificationResult,
		&j.Forced, &j.EncodePreset, &j.EncodeCRF, &j.EncodeExtraArgs,
		&j.PredictedSavingsBytes, &j.InitialEstimatedDurationSeconds,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// jobWithPathQ is jobQ plus the originating media file's path via LEFT JOIN.
// Columns shared between the two tables (id, status) are qualified to avoid
// ambiguity.
const jobWithPathQ = `
	SELECT j.id, j.media_file_id, j.profile_id, j.status, j.queued_at, j.started_at, j.completed_at,
		j.original_size_bytes, j.output_size_bytes, j.progress_percent, j.output_path,
		j.error_message, j.verification_result, j.forced,
		j.encode_preset, j.encode_crf, j.encode_extra_args,
		j.predicted_savings_bytes, j.initial_estimated_duration_seconds,
		m.path, m.duration_seconds, m.width, m.height
	FROM transcode_jobs j
	LEFT JOIN media_files m ON m.id = j.media_file_id`

// LearnedRatioMinSamples is the minimum number of completed jobs required for
// a codec before its observed ratio overrides the seed value. A small sample
// can swing wildly, so we wait for at least this many data points.
const LearnedRatioMinSamples = 10

// learnedRatioMin and learnedRatioMax clamp observed ratios to a sane range so
// a handful of unusual encodes can't produce absurd savings predictions.
const (
	learnedRatioMin = 0.30
	learnedRatioMax = 0.95
)

// LearnedRatio is the observed output/original size ratio for a source codec,
// derived from completed jobs on this instance.
type LearnedRatio struct {
	Ratio       float64
	SampleCount int
}

// LearnedRatios computes the observed mean output/original ratio per source
// video codec from completed transcode jobs that have both size fields set.
// Only codecs with at least minSamples jobs are returned. Results are clamped
// to [learnedRatioMin, learnedRatioMax].
func (j *Jobs) LearnedRatios(ctx context.Context, minSamples int) (map[string]LearnedRatio, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT LOWER(COALESCE(m.video_codec, '')),
		       COUNT(*),
		       AVG(CAST(tj.output_size_bytes AS REAL) / CAST(tj.original_size_bytes AS REAL))
		FROM transcode_jobs tj
		JOIN media_files m ON m.id = tj.media_file_id
		WHERE tj.status = 'completed'
		  AND tj.output_size_bytes IS NOT NULL
		  AND tj.output_size_bytes > 0
		  AND tj.original_size_bytes > 0
		  AND m.video_codec IS NOT NULL
		  AND m.video_codec != ''
		GROUP BY LOWER(m.video_codec)
		HAVING COUNT(*) >= ?`,
		minSamples,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]LearnedRatio)
	for rows.Next() {
		var codec string
		var count int
		var ratio float64
		if err := rows.Scan(&codec, &count, &ratio); err != nil {
			return nil, err
		}
		if ratio < learnedRatioMin {
			ratio = learnedRatioMin
		}
		if ratio > learnedRatioMax {
			ratio = learnedRatioMax
		}
		out[codec] = LearnedRatio{Ratio: ratio, SampleCount: count}
	}
	return out, rows.Err()
}

const (
	LearnedEncodeProfileMinSamples   = 3
	LearnedEncodePresetCRFMinSamples = 5
	LearnedEncodePresetMinSamples    = 5
	LearnedEncodeGlobalMinSamples    = 10
)

// LearnedEncodeRates aggregates normalized encode speeds from completed jobs,
// bucketed for the profile-first fallback cascade in media.ResolveEncodeRate.
func (j *Jobs) LearnedEncodeRates(ctx context.Context) (*media.EncodeRateLookup, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT j.profile_id,
		       COALESCE(j.encode_preset, p.preset),
		       COALESCE(j.encode_crf, p.crf),
		       j.started_at,
		       j.completed_at,
		       m.duration_seconds,
		       m.width,
		       m.height
		FROM transcode_jobs j
		JOIN media_files m ON m.id = j.media_file_id
		LEFT JOIN transcode_profiles p ON p.id = j.profile_id
		WHERE j.status = 'completed'
		  AND j.started_at IS NOT NULL
		  AND j.completed_at IS NOT NULL
		  AND j.completed_at > j.started_at
		  AND m.duration_seconds IS NOT NULL
		  AND m.duration_seconds > 0
		  AND COALESCE(j.encode_preset, p.preset) IS NOT NULL
		  AND COALESCE(j.encode_crf, p.crf) IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bucket struct {
		sum   float64
		count int
	}

	byProfile := make(map[int64]*bucket)
	byPresetCRF := make(map[string]*bucket)
	byPreset := make(map[string]*bucket)
	var global bucket

	add := func(m map[int64]*bucket, key int64, rate float64) {
		b, ok := m[key]
		if !ok {
			b = &bucket{}
			m[key] = b
		}
		b.sum += rate
		b.count++
	}
	addStr := func(m map[string]*bucket, key string, rate float64) {
		b, ok := m[key]
		if !ok {
			b = &bucket{}
			m[key] = b
		}
		b.sum += rate
		b.count++
	}

	for rows.Next() {
		var profileID int64
		var preset string
		var crf int
		var startedAt, completedAt int64
		var durationSeconds float64
		var width, height sql.NullInt64
		if err := rows.Scan(&profileID, &preset, &crf, &startedAt, &completedAt,
			&durationSeconds, &width, &height); err != nil {
			return nil, err
		}
		var w, h *int
		if width.Valid {
			v := int(width.Int64)
			w = &v
		}
		if height.Valid {
			v := int(height.Int64)
			h = &v
		}
		elapsed := completedAt - startedAt
		rate, ok := media.NormalizedEncodeRate(elapsed, durationSeconds, w, h)
		if !ok {
			continue
		}
		add(byProfile, profileID, rate)
		addStr(byPresetCRF, media.PresetCRFKey(preset, crf), rate)
		addStr(byPreset, strings.ToLower(preset), rate)
		global.sum += rate
		global.count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	toRate := func(b *bucket, min int) (media.LearnedEncodeRate, bool) {
		if b == nil || b.count < min {
			return media.LearnedEncodeRate{}, false
		}
		return media.LearnedEncodeRate{
			Rate:        media.ClampEncodeRate(b.sum / float64(b.count)),
			SampleCount: b.count,
		}, true
	}

	lookup := &media.EncodeRateLookup{
		ByProfileID: make(map[int64]media.LearnedEncodeRate),
		ByPresetCRF: make(map[string]media.LearnedEncodeRate),
		ByPreset:    make(map[string]media.LearnedEncodeRate),
	}
	for id, b := range byProfile {
		if lr, ok := toRate(b, LearnedEncodeProfileMinSamples); ok {
			lookup.ByProfileID[id] = lr
		}
	}
	for key, b := range byPresetCRF {
		if lr, ok := toRate(b, LearnedEncodePresetCRFMinSamples); ok {
			lookup.ByPresetCRF[key] = lr
		}
	}
	for key, b := range byPreset {
		if lr, ok := toRate(b, LearnedEncodePresetMinSamples); ok {
			lookup.ByPreset[key] = lr
		}
	}
	if lr, ok := toRate(&global, LearnedEncodeGlobalMinSamples); ok {
		lookup.Global = &lr
	}
	return lookup, nil
}

func scanJobWithPath(s rowScanner) (*TranscodeJob, error) {
	var j TranscodeJob
	err := s.Scan(
		&j.ID, &j.MediaFileID, &j.ProfileID, &j.Status, &j.QueuedAt,
		&j.StartedAt, &j.CompletedAt, &j.OriginalSizeBytes, &j.OutputSizeBytes,
		&j.ProgressPercent, &j.OutputPath, &j.ErrorMessage, &j.VerificationResult,
		&j.Forced, &j.EncodePreset, &j.EncodeCRF, &j.EncodeExtraArgs,
		&j.PredictedSavingsBytes, &j.InitialEstimatedDurationSeconds,
		&j.SourcePath, &j.DurationSeconds, &j.Width, &j.Height,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}
