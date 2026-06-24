-- +goose Up
-- events is an append-only audit log. Rows are written inside the same DB
-- transaction as the state change that triggered them (job completed/failed/
-- cancelled) or immediately after (scan completed, orphan restored).
CREATE TABLE events (
    id         INTEGER PRIMARY KEY,
    type       TEXT    NOT NULL,  -- job_completed|job_failed|job_cancelled|scan_completed|orphan_restored
    severity   TEXT    NOT NULL,  -- info|warn|error
    message    TEXT    NOT NULL,
    metadata   TEXT,              -- nullable JSON blob
    created_at INTEGER NOT NULL
);
CREATE INDEX events_created_at_idx ON events(created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS events_created_at_idx;
DROP TABLE IF EXISTS events;
