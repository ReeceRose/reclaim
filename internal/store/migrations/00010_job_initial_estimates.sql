-- +goose Up
-- +goose StatementBegin
ALTER TABLE transcode_jobs ADD COLUMN predicted_savings_bytes INTEGER;
ALTER TABLE transcode_jobs ADD COLUMN initial_estimated_duration_seconds INTEGER;
-- +goose StatementEnd

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; this is a no-op rollback.
