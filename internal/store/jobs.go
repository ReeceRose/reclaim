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
	// Snapshot of encode settings at queue time, restamped by the worker with
	// the live profile it actually encodes with — so learning, the ledger, and
	// post-swap crash recovery all see what really ran.
	EncodeCodec     *string
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
			encode_codec, encode_preset, encode_crf, encode_extra_args,
			predicted_savings_bytes, initial_estimated_duration_seconds, queue_order
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			(SELECT COALESCE(MAX(queue_order), 0) + 1 FROM transcode_jobs))`,
		job.MediaFileID, job.ProfileID, job.Status, job.QueuedAt, job.OriginalSizeBytes,
		job.EncodeCodec, job.EncodePreset, job.EncodeCRF, job.EncodeExtraArgs,
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

// JobFilter narrows a job list by properties of the job and its media file.
// Zero values mean "no filter". Library type and video codec are read off the
// media row, which for a job that has not run yet still describes the source.
type JobFilter struct {
	Search      string // case-insensitive substring match against the path
	LibraryType string
	VideoCodec  string
	ProfileID   int64
	ForcedOnly  bool
}

// Active reports whether any field narrows the list.
func (f JobFilter) Active() bool {
	return strings.TrimSpace(f.Search) != "" || f.LibraryType != "" ||
		f.VideoCodec != "" || f.ProfileID != 0 || f.ForcedOnly
}

// JobListQuery pages the combined queue + history list.
type JobListQuery struct {
	// Statuses, when non-empty, restricts results to jobs whose status is
	// any of these values (SQL IN).
	Statuses []string
	Filter   JobFilter
	// OrderBy selects sort order: "" (default) orders by queue_order,
	// matching queue position order; "recent" orders newest-first
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

	where, args := jobListWhere(q.Statuses, q.Filter)
	query := jobWithPathQ + where
	if q.OrderBy == "recent" {
		query += " ORDER BY COALESCE(j.completed_at, j.queued_at) DESC, j.id DESC"
	} else {
		query += " ORDER BY " + queueOrderSQL
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

// queueOrderSQL is the order the worker drains the queue in. Every query that
// lists or claims queued jobs must use it, or positions and claims disagree.
const queueOrderSQL = "j.queue_order ASC, j.id ASC"

// jobListWhere builds the WHERE clause shared by ListWithPath and CountJobs,
// over jobWithPathQ's aliases (j = transcode_jobs, m = media_files).
func jobListWhere(statuses []string, f JobFilter) (string, []any) {
	where := " WHERE j.dismissed_at IS NULL"
	var args []any
	if len(statuses) > 0 {
		where += " AND j.status IN (" + placeholders(len(statuses)) + ")"
		for _, s := range statuses {
			args = append(args, s)
		}
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		where += " AND LOWER(COALESCE(m.path, j.output_path, '')) LIKE '%' || LOWER(?) || '%'"
		args = append(args, s)
	}
	if f.LibraryType != "" {
		where += " AND m.library_type = ?"
		args = append(args, f.LibraryType)
	}
	if f.VideoCodec != "" {
		where += " AND m.video_codec = ?"
		args = append(args, f.VideoCodec)
	}
	if f.ProfileID != 0 {
		where += " AND j.profile_id = ?"
		args = append(args, f.ProfileID)
	}
	if f.ForcedOnly {
		where += " AND j.forced = 1"
	}
	return where, args
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// CountJobs returns how many non-dismissed jobs match an optional status
// filter and JobFilter.
func (j *Jobs) CountJobs(ctx context.Context, statuses []string, f JobFilter) (int64, error) {
	where, args := jobListWhere(statuses, f)
	query := `SELECT COUNT(*) FROM transcode_jobs j LEFT JOIN media_files m ON m.id = j.media_file_id` + where
	var n int64
	if err := j.r.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// HistorySummary aggregates every finished job the history view can show, so
// the UI can report totals without paging through the whole list. Byte figures
// cover completed jobs only: a failed job never swapped a file, so it has no
// output size to weigh in.
type HistorySummary struct {
	CompletedCount    int64
	FailedCount       int64
	CancelledCount    int64
	OriginalSizeBytes int64
	OutputSizeBytes   int64
	EncodeSeconds     int64
}

// HistorySummary returns those totals over non-dismissed jobs matching an
// optional status filter.
func (j *Jobs) HistorySummary(ctx context.Context, statuses []string) (HistorySummary, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN original_size_bytes ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN COALESCE(output_size_bytes, original_size_bytes) ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'completed' AND started_at IS NOT NULL AND completed_at > started_at
				THEN completed_at - started_at ELSE 0 END), 0)
		FROM transcode_jobs WHERE dismissed_at IS NULL`
	var args []any
	if len(statuses) > 0 {
		query += " AND status IN (" + placeholders(len(statuses)) + ")"
		for _, s := range statuses {
			args = append(args, s)
		}
	}
	var h HistorySummary
	err := j.r.QueryRowContext(ctx, query, args...).Scan(
		&h.CompletedCount, &h.FailedCount, &h.CancelledCount,
		&h.OriginalSizeBytes, &h.OutputSizeBytes, &h.EncodeSeconds,
	)
	if err != nil {
		return HistorySummary{}, err
	}
	return h, nil
}

// QueuedPositions returns 1-based queue positions for every queued job.
func (j *Jobs) QueuedPositions(ctx context.Context) (map[int64]int, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT j.id FROM transcode_jobs j
		WHERE j.status = 'queued'
		ORDER BY `+queueOrderSQL)
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
		jobQ+" j WHERE status = 'queued' ORDER BY "+queueOrderSQL+" LIMIT 1"))
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
// SetEncodeSettings restamps a job's encode snapshot with the settings the
// worker is about to run. The worker encodes with the live profile, which may
// have been edited — even switched codec — since the job was queued; recording
// what actually ran keeps encode-time learning, the savings ledger's result
// codec, and post-swap crash recovery honest.
func (j *Jobs) SetEncodeSettings(ctx context.Context, id int64, codec, preset string, crf int, extraArgs *string) error {
	_, err := j.w.ExecContext(ctx, `
		UPDATE transcode_jobs
		SET encode_codec = ?, encode_preset = ?, encode_crf = ?, encode_extra_args = ?
		WHERE id = ?`,
		codec, preset, crf, extraArgs, id,
	)
	return err
}

// encodeCodecTx resolves the codec a job encodes to: its snapshot, else its
// profile's, else the default.
func (j *Jobs) encodeCodecTx(ctx context.Context, tx *sql.Tx, id int64) (string, error) {
	var codec string
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(j.encode_codec, ''), p.codec, '')
		FROM transcode_jobs j
		LEFT JOIN transcode_profiles p ON p.id = j.profile_id
		WHERE j.id = ?`, id,
	).Scan(&codec)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(media.NormalizeTargetCodec(codec)), nil
}

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

// QueuedIDs returns the ids of every queued job matching f, in queue order. A
// zero filter matches the whole queue.
func (j *Jobs) QueuedIDs(ctx context.Context, f JobFilter) ([]int64, error) {
	where, args := jobListWhere([]string{string(jobs.StatusQueued)}, f)
	rows, err := j.r.QueryContext(ctx,
		`SELECT j.id FROM transcode_jobs j LEFT JOIN media_files m ON m.id = j.media_file_id`+
			where+" ORDER BY "+queueOrderSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MoveQueued moves the given queued jobs to the front (toTop) or back of the
// queue, keeping their relative order. Ids that are not queued — already
// claimed, cancelled, or unknown — are skipped, and the number actually moved
// is returned. The running job is unaffected: it has already left the queue.
func (j *Jobs) MoveQueued(ctx context.Context, ids []int64, toTop bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := j.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	slots, err := queuedSlotsTx(ctx, tx, ids)
	if err != nil {
		return 0, err
	}
	if len(slots) == 0 {
		return 0, nil
	}
	moving := make([]int64, len(slots))
	for i, sl := range slots {
		moving[i] = sl.id
	}

	// Top counts down from the smallest queued position; bottom counts up from
	// the largest position of any row, so a later Create's MAX+1 still lands
	// behind everything moved here.
	var start int64
	if toTop {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MIN(queue_order), 0) FROM transcode_jobs WHERE status = 'queued'`,
		).Scan(&start); err != nil {
			return 0, err
		}
		start -= int64(len(moving))
	} else {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(queue_order), 0) FROM transcode_jobs`,
		).Scan(&start); err != nil {
			return 0, err
		}
		start++
	}

	orders := make([]int64, len(moving))
	for i := range orders {
		orders[i] = start + int64(i)
	}
	if err := setQueueOrdersTx(ctx, tx, moving, orders); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(moving), nil
}

// StepQueued moves each of the given queued jobs one place up (or down) the
// whole queue, swapping it with its neighbour. A run of selected jobs moves as
// a block, and one already at the end it is heading for stays put, so a
// selection never overtakes itself. Ids that are not queued are skipped;
// the number of jobs that actually moved is returned.
func (j *Jobs) StepQueued(ctx context.Context, ids []int64, up bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	selected := make(map[int64]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}

	tx, err := j.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT j.id, j.queue_order FROM transcode_jobs j WHERE j.status = 'queued' ORDER BY `+queueOrderSQL)
	if err != nil {
		return 0, err
	}
	var seq []int64
	var orders []int64
	for rows.Next() {
		var id, order int64
		if err := rows.Scan(&id, &order); err != nil {
			rows.Close()
			return 0, err
		}
		seq = append(seq, id)
		orders = append(orders, order)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	moved := 0
	if up {
		for i := 1; i < len(seq); i++ {
			if selected[seq[i]] && !selected[seq[i-1]] {
				seq[i-1], seq[i] = seq[i], seq[i-1]
				moved++
			}
		}
	} else {
		for i := len(seq) - 2; i >= 0; i-- {
			if selected[seq[i]] && !selected[seq[i+1]] {
				seq[i], seq[i+1] = seq[i+1], seq[i]
				moved++
			}
		}
	}
	if moved == 0 {
		return 0, nil
	}
	if err := setQueueOrdersTx(ctx, tx, seq, orders); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return moved, nil
}

// ApplyQueueOrder reorders the given queued jobs among themselves: ids is the
// order they should run in, and they are dealt back into the positions they
// already hold between them. Jobs outside ids keep their places, so sorting
// one show's episodes leaves the rest of the queue interleaved as it was.
// Ids that are no longer queued are skipped; the number reordered is returned.
func (j *Jobs) ApplyQueueOrder(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := j.w.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	slots, err := queuedSlotsTx(ctx, tx, ids)
	if err != nil {
		return 0, err
	}
	if len(slots) == 0 {
		return 0, nil
	}
	orderOf := make(map[int64]int64, len(slots))
	orders := make([]int64, len(slots))
	for i, sl := range slots {
		orderOf[sl.id] = sl.order
		orders[i] = sl.order
	}
	ordered := make([]int64, 0, len(slots))
	for _, id := range ids {
		if _, ok := orderOf[id]; ok {
			ordered = append(ordered, id)
			delete(orderOf, id)
		}
	}
	if err := setQueueOrdersTx(ctx, tx, ordered, orders); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ordered), nil
}

type queueSlot struct {
	id, order int64
}

// queuedSlotsTx returns which of ids are still queued, with their current
// queue_order, in queue order.
func queuedSlotsTx(ctx context.Context, tx *sql.Tx, ids []int64) ([]queueSlot, error) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT j.id, j.queue_order FROM transcode_jobs j WHERE j.status = 'queued' AND j.id IN (`+
			placeholders(len(ids))+`) ORDER BY `+queueOrderSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []queueSlot
	for rows.Next() {
		var sl queueSlot
		if err := rows.Scan(&sl.id, &sl.order); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

// setQueueOrdersTx writes orders[i] to ids[i].
func setQueueOrdersTx(ctx context.Context, tx *sql.Tx, ids, orders []int64) error {
	stmt, err := tx.PrepareContext(ctx, `UPDATE transcode_jobs SET queue_order = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, orders[i], id); err != nil {
			return err
		}
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
		jobQ+" j WHERE status = 'queued' AND forced = 1 ORDER BY "+queueOrderSQL+" LIMIT 1"))
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
		predicted_savings_bytes, initial_estimated_duration_seconds, encode_codec
	FROM transcode_jobs`

func scanJob(s rowScanner) (*TranscodeJob, error) {
	var j TranscodeJob
	err := s.Scan(
		&j.ID, &j.MediaFileID, &j.ProfileID, &j.Status, &j.QueuedAt,
		&j.StartedAt, &j.CompletedAt, &j.OriginalSizeBytes, &j.OutputSizeBytes,
		&j.ProgressPercent, &j.OutputPath, &j.ErrorMessage, &j.VerificationResult,
		&j.Forced, &j.EncodePreset, &j.EncodeCRF, &j.EncodeExtraArgs,
		&j.PredictedSavingsBytes, &j.InitialEstimatedDurationSeconds, &j.EncodeCodec,
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
		j.predicted_savings_bytes, j.initial_estimated_duration_seconds, j.encode_codec,
		m.path, m.duration_seconds, m.width, m.height
	FROM transcode_jobs j
	LEFT JOIN media_files m ON m.id = j.media_file_id`

// LearnedRatioMinSamples is the minimum number of completed jobs required for
// a codec before its observed ratio overrides the seed value. A small sample
// can swing wildly, so we wait for at least this many data points.
const LearnedRatioMinSamples = 10

// learnedRatioMin and learnedRatioMax clamp observed ratios to a sane range so
// a handful of unusual encodes can't produce absurd savings predictions. The
// floor sits below what real h264 sources reach at typical CRFs (~0.35), so it
// bounds outliers rather than capping an ordinary library.
const (
	learnedRatioMin = 0.20
	learnedRatioMax = 0.95
)

// LearnedRatio is the observed output/original size ratio for a source codec
// encoded to one target codec, derived from completed jobs on this instance.
type LearnedRatio struct {
	Ratio       float64
	SampleCount int
}

// LearnedRatios computes the observed output/original ratio per source video
// codec, for encodes to target, from the savings ledger. Only codecs with at
// least minSamples encodes are returned. Results are clamped to
// [learnedRatioMin, learnedRatioMax]. Targets are never pooled: an h264 file
// shrinks further under SVT-AV1 than under x265, so an HEVC ratio would
// under-promise AV1 savings and an AV1 ratio would over-promise HEVC's.
//
// The ratio is byte-weighted (total output over total original) rather than a
// mean of per-file ratios. Predictions are summed into the library's remaining
// savings and scored against realized bytes on the Insights page, and large
// sources tend to compress further than small ones, so an unweighted mean
// systematically under-predicts the total.
//
// This reads savings_ledger rather than joining transcode_jobs to media_files
// because the swap rewrites video_codec to the target, so the post-encode media row
// no longer knows what the source codec was. Rows backfilled from job history
// predate the ledger and carry no source codec, so they never contribute.
//
// Replacement rows are excluded. They share the table but not the meaning: the
// size ratio between a deleted release and the one downloaded to replace it
// says nothing about what this encoder achieves at this CRF, and folding it in
// would train the savings model on someone else's encode.
func (j *Jobs) LearnedRatios(ctx context.Context, target media.TargetCodec, minSamples int) (map[string]LearnedRatio, error) {
	rows, err := j.r.QueryContext(ctx, `
		SELECT LOWER(source_codec),
		       COUNT(*),
		       CAST(SUM(output_size_bytes) AS REAL) / CAST(SUM(original_size_bytes) AS REAL)
		FROM savings_ledger
		WHERE source = 'encode'
		  AND LOWER(COALESCE(NULLIF(result_codec, ''), ?)) = ?
		  AND source_codec IS NOT NULL
		  AND source_codec != ''
		  AND output_size_bytes > 0
		  AND original_size_bytes > 0
		GROUP BY LOWER(source_codec)
		HAVING COUNT(*) >= ?`,
		string(media.DefaultTargetCodec), string(target), minSamples,
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
		       COALESCE(NULLIF(j.encode_codec, ''), p.codec, ''),
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

	byProfile := make(map[string]*bucket)
	byPresetCRF := make(map[string]*bucket)
	byPreset := make(map[string]*bucket)
	byCodec := make(map[string]*bucket)

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
		var rawCodec, preset string
		var crf int
		var startedAt, completedAt int64
		var durationSeconds float64
		var width, height sql.NullInt64
		if err := rows.Scan(&profileID, &rawCodec, &preset, &crf, &startedAt, &completedAt,
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
		codec := media.NormalizeTargetCodec(rawCodec)
		addStr(byProfile, media.ProfileRateKey(profileID, codec), rate)
		addStr(byPresetCRF, media.PresetCRFKey(codec, preset, crf), rate)
		addStr(byPreset, media.PresetKey(codec, preset), rate)
		addStr(byCodec, string(codec), rate)
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
		ByProfile:   make(map[string]media.LearnedEncodeRate),
		ByPresetCRF: make(map[string]media.LearnedEncodeRate),
		ByPreset:    make(map[string]media.LearnedEncodeRate),
		ByCodec:     make(map[string]media.LearnedEncodeRate),
	}
	for key, b := range byProfile {
		if lr, ok := toRate(b, LearnedEncodeProfileMinSamples); ok {
			lookup.ByProfile[key] = lr
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
	for key, b := range byCodec {
		if lr, ok := toRate(b, LearnedEncodeGlobalMinSamples); ok {
			lookup.ByCodec[key] = lr
		}
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
		&j.PredictedSavingsBytes, &j.InitialEstimatedDurationSeconds, &j.EncodeCodec,
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
