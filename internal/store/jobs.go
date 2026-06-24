package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
}

type Jobs struct {
	r, w *sql.DB
}

func (j *Jobs) Create(ctx context.Context, job *TranscodeJob) (int64, error) {
	res, err := j.w.ExecContext(ctx, `
		INSERT INTO transcode_jobs (media_file_id, profile_id, status, queued_at, original_size_bytes)
		VALUES (?, ?, ?, ?, ?)`,
		job.MediaFileID, job.ProfileID, job.Status, job.QueuedAt, job.OriginalSizeBytes,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (j *Jobs) GetByID(ctx context.Context, id int64) (*TranscodeJob, error) {
	return scanJob(j.r.QueryRowContext(ctx, jobQ+" WHERE id = ?", id))
}

func (j *Jobs) ListByStatus(ctx context.Context, status string) ([]TranscodeJob, error) {
	rows, err := j.r.QueryContext(ctx, jobQ+" WHERE status = ? ORDER BY queued_at", status)
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

// ListAll returns every job, newest first. Used by GET /api/jobs to render the
// combined queue + history view.
func (j *Jobs) ListAll(ctx context.Context) ([]TranscodeJob, error) {
	rows, err := j.r.QueryContext(ctx, jobQ+" ORDER BY queued_at DESC, id DESC")
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

func (j *Jobs) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET status = ? WHERE id = ?", status, id,
	)
	return err
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

	job.Status = "running"
	job.StartedAt = &startedAt
	return job, nil
}

// Transition performs a guarded status change: it only succeeds if the row is
// currently in `from`, which both serializes through the single writer and
// rejects illegal transitions (the FSM guard lives in jobs.CanTransition; this
// is the persistence-layer enforcement of it). Returns ErrIllegalTransition if
// the row was not in the expected state.
func (j *Jobs) Transition(ctx context.Context, id int64, from, to string) error {
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

// SetVerificationResult stores the verification JSON blob.
func (j *Jobs) SetVerificationResult(ctx context.Context, id int64, result string) error {
	_, err := j.w.ExecContext(ctx,
		"UPDATE transcode_jobs SET verification_result = ? WHERE id = ?", result, id,
	)
	return err
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
		error_message, verification_result
	FROM transcode_jobs`

func scanJob(s rowScanner) (*TranscodeJob, error) {
	var j TranscodeJob
	err := s.Scan(
		&j.ID, &j.MediaFileID, &j.ProfileID, &j.Status, &j.QueuedAt,
		&j.StartedAt, &j.CompletedAt, &j.OriginalSizeBytes, &j.OutputSizeBytes,
		&j.ProgressPercent, &j.OutputPath, &j.ErrorMessage, &j.VerificationResult,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}
