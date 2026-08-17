-- +goose Up
-- +goose StatementBegin
ALTER TABLE settings ADD COLUMN notify_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE settings ADD COLUMN notify_delay_seconds INTEGER NOT NULL DEFAULT 900;
ALTER TABLE settings ADD COLUMN notify_webhook_url TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN notify_webhook_format TEXT NOT NULL DEFAULT 'json';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE settings DROP COLUMN notify_webhook_format;
ALTER TABLE settings DROP COLUMN notify_webhook_url;
ALTER TABLE settings DROP COLUMN notify_delay_seconds;
ALTER TABLE settings DROP COLUMN notify_enabled;
-- +goose StatementEnd
