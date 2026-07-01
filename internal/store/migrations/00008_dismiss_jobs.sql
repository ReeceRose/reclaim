-- +goose Up
ALTER TABLE transcode_jobs ADD COLUMN dismissed_at INTEGER;

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; this is a no-op rollback.
