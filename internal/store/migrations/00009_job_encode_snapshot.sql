-- +goose Up
-- +goose StatementBegin
ALTER TABLE transcode_jobs ADD COLUMN encode_preset TEXT;
ALTER TABLE transcode_jobs ADD COLUMN encode_crf INTEGER;
ALTER TABLE transcode_jobs ADD COLUMN encode_extra_args TEXT;

-- Backfill from current profile settings (imperfect for edited profiles, better than NULL).
UPDATE transcode_jobs
SET encode_preset = (SELECT preset FROM transcode_profiles WHERE id = transcode_jobs.profile_id),
    encode_crf = (SELECT crf FROM transcode_profiles WHERE id = transcode_jobs.profile_id),
    encode_extra_args = (SELECT extra_args FROM transcode_profiles WHERE id = transcode_jobs.profile_id)
WHERE encode_preset IS NULL;
-- +goose StatementEnd

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; this is a no-op rollback.
