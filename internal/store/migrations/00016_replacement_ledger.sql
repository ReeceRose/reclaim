-- +goose Up
ALTER TABLE media_files ADD COLUMN replace_key TEXT NOT NULL DEFAULT '';

-- Stamped by the backfill so a row whose identity simply cannot be derived (a
-- TV file with no SxxExx token) is attempted once rather than re-walked on
-- every boot. An empty key with a stamp means "tried, unidentifiable".
ALTER TABLE media_files ADD COLUMN last_keyed_at INTEGER;

-- When this row was first indexed. The arrival side of a replacement match is
-- bounded by it: without an arrival time, deleting one of two long-standing
-- copies of the same title (a 1080p and a 4K cut kept side by side) would see
-- the survivor as the replacement and book a fabricated delta. last_probed_at
-- cannot stand in — a re-probe moves it, making an old file look new.
--
-- Existing rows are seeded from last_probed_at, which for an unchanged file is
-- the scan that first indexed it. That is an approximation, but it only ever
-- makes an old row look older than the lookback and therefore ineligible,
-- which is the safe direction.
ALTER TABLE media_files ADD COLUMN first_seen_at INTEGER;
UPDATE media_files SET first_seen_at = last_probed_at;

-- Both sides of a delete-and-redownload pair are looked up by key: the missing
-- row when its replacement is indexed, and the surviving row when the watcher
-- sees the delete after the import. Rows with no derivable identity are
-- excluded so the index stays proportional to what is actually matchable.
CREATE INDEX IF NOT EXISTS media_files_replace_key_idx
    ON media_files(replace_key, status)
    WHERE replace_key != '';

-- savings_ledger becomes the ledger of bytes reclaimed by any means, not just
-- by an encode job: job_id has to go nullable, which SQLite can only do by
-- rebuilding the table. The old indexes travel with the rename, so they are
-- dropped before the new table claims their names.
DROP INDEX IF EXISTS savings_ledger_source_codec_idx;
DROP INDEX IF EXISTS savings_ledger_completed_at_idx;
ALTER TABLE savings_ledger RENAME TO savings_ledger_old;

-- +goose StatementBegin
CREATE TABLE savings_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL DEFAULT 'encode',
    match_kind TEXT,
    job_id INTEGER,
    media_file_id INTEGER NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    previous_path TEXT,
    library_type TEXT NOT NULL DEFAULT 'unknown',
    source_codec TEXT,
    result_codec TEXT,
    width INTEGER,
    height INTEGER,
    result_width INTEGER,
    result_height INTEGER,
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

-- Partial unique: one row per encode job (what makes RecordTx's INSERT OR
-- IGNORE idempotent against a replayed commit), while replacement rows all
-- carry a NULL job_id and must not collide with each other.
CREATE UNIQUE INDEX IF NOT EXISTS savings_ledger_job_id_idx
    ON savings_ledger(job_id) WHERE job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS savings_ledger_completed_at_idx ON savings_ledger(completed_at DESC);
CREATE INDEX IF NOT EXISTS savings_ledger_source_codec_idx ON savings_ledger(source_codec);
CREATE INDEX IF NOT EXISTS savings_ledger_source_idx ON savings_ledger(source, completed_at DESC);

-- +goose StatementBegin
INSERT INTO savings_ledger (
    source, job_id, media_file_id, path, library_type, source_codec,
    width, height, duration_seconds,
    original_size_bytes, output_size_bytes,
    predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
    profile_id, encode_preset, encode_crf, completed_at
)
SELECT 'encode', job_id, media_file_id, path, library_type, source_codec,
       width, height, duration_seconds,
       original_size_bytes, output_size_bytes,
       predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
       profile_id, encode_preset, encode_crf, completed_at
FROM savings_ledger_old;
-- +goose StatementEnd

DROP TABLE savings_ledger_old;

-- +goose Down
DROP INDEX IF EXISTS savings_ledger_source_idx;
DROP INDEX IF EXISTS savings_ledger_source_codec_idx;
DROP INDEX IF EXISTS savings_ledger_completed_at_idx;
DROP INDEX IF EXISTS savings_ledger_job_id_idx;
ALTER TABLE savings_ledger RENAME TO savings_ledger_new;

-- +goose StatementBegin
CREATE TABLE savings_ledger (
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
INSERT INTO savings_ledger (
    job_id, media_file_id, path, library_type, source_codec,
    width, height, duration_seconds,
    original_size_bytes, output_size_bytes,
    predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
    profile_id, encode_preset, encode_crf, completed_at
)
SELECT job_id, media_file_id, path, library_type, source_codec,
       width, height, duration_seconds,
       original_size_bytes, output_size_bytes,
       predicted_savings_bytes, estimated_duration_seconds, encode_seconds,
       profile_id, encode_preset, encode_crf, completed_at
FROM savings_ledger_new
WHERE source = 'encode' AND job_id IS NOT NULL;
-- +goose StatementEnd

DROP TABLE savings_ledger_new;

DROP INDEX IF EXISTS media_files_replace_key_idx;
ALTER TABLE media_files DROP COLUMN first_seen_at;
ALTER TABLE media_files DROP COLUMN last_keyed_at;
ALTER TABLE media_files DROP COLUMN replace_key;
