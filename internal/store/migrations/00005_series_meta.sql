-- +goose Up
ALTER TABLE media_files ADD COLUMN series_title TEXT;
ALTER TABLE media_files ADD COLUMN season_number INTEGER;

CREATE INDEX IF NOT EXISTS idx_media_files_series_title
    ON media_files(series_title)
    WHERE library_type = 'tv' AND series_title IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_media_files_series_title;
