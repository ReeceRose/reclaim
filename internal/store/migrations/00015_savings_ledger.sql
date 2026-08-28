-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS savings_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL UNIQUE,
    media_file_id INTEGER NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    library_type TEXT NOT NULL DEFAULT 'unknown',
    source_codec TEXT,
    width INTEGER,
    height INTEGER,
    duration_seconds REAL,
    original_size_bytes INTEGER NOT NULL,
    output_size_bytes INTEGER NOT NULL,
    predicted_savings_bytes INTEGER,
    estimated_duration_seconds INTEGER,
    encode_seconds INTEGER,
    profile_id INTEGER,
    encode_preset TEXT,
    encode_crf INTEGER,
    completed_at INTEGER NOT NULL
);
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS savings_ledger_completed_at_idx ON savings_ledger(completed_at DESC);
CREATE INDEX IF NOT EXISTS savings_ledger_source_codec_idx ON savings_ledger(source_codec);

-- +goose StatementBegin
INSERT OR IGNORE INTO savings_ledger (
    job_id, media_file_id, path, library_type, source_codec,
    width, height, duration_seconds,
    original_size_bytes, output_size_bytes,
    predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
    profile_id, encode_preset, encode_crf, completed_at
)
SELECT j.id, j.media_file_id,
       COALESCE(m.path, ''), COALESCE(m.library_type, 'unknown'), NULL,
       m.width, m.height, m.duration_seconds,
       j.original_size_bytes, j.output_size_bytes,
       j.predicted_savings_bytes, j.initial_estimated_duration_seconds,
       CASE WHEN j.started_at IS NOT NULL AND j.completed_at > j.started_at
            THEN j.completed_at - j.started_at END,
       j.profile_id, j.encode_preset, j.encode_crf, j.completed_at
FROM transcode_jobs j
LEFT JOIN media_files m ON m.id = j.media_file_id
WHERE j.status = 'completed'
  AND j.completed_at IS NOT NULL
  AND j.output_size_bytes IS NOT NULL
  AND j.output_size_bytes > 0
  AND j.original_size_bytes > 0;
-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS savings_ledger_source_codec_idx;
DROP INDEX IF EXISTS savings_ledger_completed_at_idx;
DROP TABLE IF EXISTS savings_ledger;
