package store

import (
	"context"
	"database/sql"
	"errors"
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
