-- +goose Up
-- +goose StatementBegin
ALTER TABLE media_files ADD COLUMN missing_since INTEGER;
-- +goose StatementEnd

-- Existing missing rows predate the column, so age them from the last time they
-- were seen on disk (last_probed_at) rather than from this migration — a row
-- that vanished eight months ago should be eligible for pruning immediately,
-- not after another full retention period.
-- +goose StatementBegin
UPDATE media_files
SET missing_since = COALESCE(last_probed_at, unixepoch())
WHERE status = 'missing' AND missing_since IS NULL;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_media_files_missing_since ON media_files(missing_since)
    WHERE status = 'missing';

-- +goose Down
DROP INDEX IF EXISTS idx_media_files_missing_since;
