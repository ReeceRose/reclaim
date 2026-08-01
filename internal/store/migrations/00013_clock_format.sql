-- +goose Up
-- +goose StatementBegin
ALTER TABLE settings ADD COLUMN clock_format TEXT NOT NULL DEFAULT '12h';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE settings DROP COLUMN clock_format;
-- +goose StatementEnd
