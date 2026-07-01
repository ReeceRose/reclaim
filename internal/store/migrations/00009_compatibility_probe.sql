-- +goose Up
-- Denormalized primary-stream fields needed by the compatibility engine
-- (internal/compatibility). Kept on media_files so the fast list queries (candidates,
-- files) never need to join media_streams just to filter/sort.
ALTER TABLE media_files ADD COLUMN pixel_format TEXT;
ALTER TABLE media_files ADD COLUMN video_bit_depth INTEGER;
ALTER TABLE media_files ADD COLUMN color_transfer TEXT;
ALTER TABLE media_files ADD COLUMN color_primaries TEXT;
ALTER TABLE media_files ADD COLUMN audio_sample_rate INTEGER;
ALTER TABLE media_files ADD COLUMN subtitle_codec TEXT;

-- media_streams holds every stream (not just the first video/audio), needed
-- for multi-audio-track, PGS-subtitle, and Atmos/DTS-HD detection that the
-- denormalized columns above can't express.
CREATE TABLE IF NOT EXISTS media_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_file_id INTEGER NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    stream_index INTEGER NOT NULL,
    codec_type TEXT NOT NULL,       -- video | audio | subtitle
    codec_name TEXT,
    profile TEXT,
    channels INTEGER,
    language TEXT,
    disposition_default INTEGER NOT NULL DEFAULT 0,
    extra_json TEXT,                -- pix_fmt, level, bit_rate, sample_rate, color_* — rare fields
    UNIQUE(media_file_id, stream_index)
);
CREATE INDEX IF NOT EXISTS idx_media_streams_file ON media_streams(media_file_id);

-- +goose Down
DROP INDEX IF EXISTS idx_media_streams_file;
DROP TABLE IF EXISTS media_streams;
ALTER TABLE media_files DROP COLUMN subtitle_codec;
ALTER TABLE media_files DROP COLUMN audio_sample_rate;
ALTER TABLE media_files DROP COLUMN color_primaries;
ALTER TABLE media_files DROP COLUMN color_transfer;
ALTER TABLE media_files DROP COLUMN video_bit_depth;
ALTER TABLE media_files DROP COLUMN pixel_format;
