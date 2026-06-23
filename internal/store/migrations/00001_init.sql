-- +goose Up
CREATE TABLE IF NOT EXISTS media_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    library_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    mtime INTEGER NOT NULL,
    fingerprint TEXT NOT NULL DEFAULT '',
    video_codec TEXT,
    video_codec_profile TEXT,
    width INTEGER,
    height INTEGER,
    duration_seconds REAL,
    bitrate_kbps INTEGER,
    audio_codec TEXT,
    audio_channels INTEGER,
    container_format TEXT,
    is_already_hevc INTEGER NOT NULL DEFAULT 0,
    predicted_savings_bytes INTEGER NOT NULL DEFAULT 0,
    last_probed_at INTEGER,
    probe_error TEXT,
    status TEXT NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS transcode_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    crf INTEGER NOT NULL,
    preset TEXT NOT NULL,
    extra_args TEXT,
    is_default INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS transcode_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_file_id INTEGER NOT NULL REFERENCES media_files(id),
    profile_id INTEGER NOT NULL REFERENCES transcode_profiles(id),
    status TEXT NOT NULL DEFAULT 'queued',
    queued_at INTEGER NOT NULL,
    started_at INTEGER,
    completed_at INTEGER,
    original_size_bytes INTEGER NOT NULL,
    output_size_bytes INTEGER,
    progress_percent REAL NOT NULL DEFAULT 0,
    output_path TEXT,
    error_message TEXT,
    verification_result TEXT
);

CREATE TABLE IF NOT EXISTS scan_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trigger TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    completed_at INTEGER,
    files_scanned INTEGER NOT NULL DEFAULT 0,
    files_added INTEGER NOT NULL DEFAULT 0,
    files_updated INTEGER NOT NULL DEFAULT 0,
    files_moved INTEGER NOT NULL DEFAULT 0,
    files_removed INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    auth_username TEXT,
    auth_password_hash TEXT,
    session_secret TEXT,
    setup_completed_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_media_files_fingerprint ON media_files(fingerprint);
CREATE INDEX IF NOT EXISTS idx_media_files_candidates ON media_files(status, is_already_hevc)
    WHERE status = 'active' AND is_already_hevc = 0;
CREATE INDEX IF NOT EXISTS idx_media_files_savings ON media_files(predicted_savings_bytes DESC, id);
CREATE INDEX IF NOT EXISTS idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX IF NOT EXISTS idx_transcode_jobs_media_file_id ON transcode_jobs(media_file_id);

INSERT OR IGNORE INTO settings (id) VALUES (1);

INSERT INTO transcode_profiles (name, crf, preset, is_default)
SELECT 'Space Saver', 26, 'medium', 1
WHERE NOT EXISTS (SELECT 1 FROM transcode_profiles WHERE is_default = 1);

-- +goose Down
DROP INDEX IF EXISTS idx_transcode_jobs_media_file_id;
DROP INDEX IF EXISTS idx_transcode_jobs_status;
DROP INDEX IF EXISTS idx_media_files_savings;
DROP INDEX IF EXISTS idx_media_files_candidates;
DROP INDEX IF EXISTS idx_media_files_fingerprint;
DROP TABLE IF EXISTS transcode_jobs;
DROP TABLE IF EXISTS transcode_profiles;
DROP TABLE IF EXISTS scan_runs;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS media_files;
