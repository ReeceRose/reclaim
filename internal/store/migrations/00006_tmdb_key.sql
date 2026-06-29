-- +goose Up
ALTER TABLE settings ADD COLUMN tmdb_api_key TEXT;

-- +goose Down
ALTER TABLE settings DROP COLUMN tmdb_api_key;
