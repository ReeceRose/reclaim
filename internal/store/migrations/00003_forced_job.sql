-- +goose Up
ALTER TABLE transcode_jobs ADD COLUMN forced BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; this is a no-op rollback.
