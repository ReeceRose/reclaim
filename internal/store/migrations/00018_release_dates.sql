-- +goose Up
ALTER TABLE media_metadata ADD COLUMN release_date TEXT;

ALTER TABLE media_files ADD COLUMN metadata_key TEXT;
ALTER TABLE media_files ADD COLUMN episode_number INTEGER;
ALTER TABLE media_files ADD COLUMN parsed_release_date TEXT;

CREATE INDEX IF NOT EXISTS media_files_metadata_key_idx
    ON media_files(metadata_key)
    WHERE metadata_key IS NOT NULL AND metadata_key != '';

-- +goose StatementBegin
CREATE TABLE episode_air_dates (
    series_key     TEXT    NOT NULL,
    season_number  INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    air_date       TEXT    NOT NULL,
    PRIMARY KEY (series_key, season_number, episode_number)
) WITHOUT ROWID;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE season_air_date_fetches (
    series_key    TEXT    NOT NULL,
    season_number INTEGER NOT NULL,
    fetched_at    INTEGER NOT NULL,
    PRIMARY KEY (series_key, season_number)
) WITHOUT ROWID;
-- +goose StatementEnd

UPDATE media_metadata SET fetched_at = 0 WHERE is_manual = 0 AND no_match = 0;

-- +goose Down
DROP TABLE IF EXISTS season_air_date_fetches;
DROP TABLE IF EXISTS episode_air_dates;
DROP INDEX IF EXISTS media_files_metadata_key_idx;
ALTER TABLE media_files DROP COLUMN parsed_release_date;
ALTER TABLE media_files DROP COLUMN episode_number;
ALTER TABLE media_files DROP COLUMN metadata_key;
ALTER TABLE media_metadata DROP COLUMN release_date;
