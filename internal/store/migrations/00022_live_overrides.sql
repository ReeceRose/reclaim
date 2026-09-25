-- +goose Up
-- +goose StatementBegin
-- The runtime knobs in config.Live (encode window, timezone, scan cadence, …)
-- were held in memory only, so a restart silently reverted anything set in the
-- UI to its env value. This holds every override the operator has made, as a
-- JSON object of the fields PUT /api/settings accepts; main.go restores it over
-- the env seed at boot. A field absent from the object still follows its env var.
ALTER TABLE settings ADD COLUMN live_overrides TEXT NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE settings DROP COLUMN live_overrides;
-- +goose StatementEnd
