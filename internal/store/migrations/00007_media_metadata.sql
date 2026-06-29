-- +goose Up
CREATE TABLE media_metadata (
    key           TEXT PRIMARY KEY,
    media_type    TEXT NOT NULL,
    tmdb_id       INTEGER,
    title         TEXT,
    tagline       TEXT,
    overview      TEXT,
    poster_path   TEXT,
    backdrop_path TEXT,
    release_year  INTEGER,
    runtime_mins  INTEGER,
    vote_average  REAL,
    vote_count    INTEGER,
    genres        TEXT,
    status        TEXT,
    collection    TEXT,
    network       TEXT,
    in_production INTEGER,
    is_manual     INTEGER NOT NULL DEFAULT 0,
    no_match      INTEGER NOT NULL DEFAULT 0,
    fetched_at    INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS media_metadata;
