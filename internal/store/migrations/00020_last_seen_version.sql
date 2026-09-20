-- +goose Up
-- +goose StatementBegin
-- Drives the "what's new" prompt after an upgrade: the app compares the running
-- build's version against this, so it has to survive restarts (config.Live is
-- re-seeded from env on boot and would forget). Empty on an existing install
-- means "never acknowledged"; main.go stamps it on a fresh one so a first-run
-- user is not greeted with a changelog.
ALTER TABLE settings ADD COLUMN last_seen_version TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE settings DROP COLUMN last_seen_version;
-- +goose StatementEnd
