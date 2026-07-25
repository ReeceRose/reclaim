-- +goose Up
-- +goose StatementBegin
ALTER TABLE media_files ADD COLUMN oversize_ratio REAL NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; this is a no-op rollback.
