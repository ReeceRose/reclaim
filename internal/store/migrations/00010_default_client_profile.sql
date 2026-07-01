-- +goose Up
-- Sticky default client profile for the compatibility view (internal/compatibility).
-- Lives in the settings singleton row, not config.Live, because it's a user
-- preference that must survive a restart — Live is re-seeded from env on boot.
ALTER TABLE settings ADD COLUMN default_client_profile TEXT NOT NULL DEFAULT 'apple_tv_4k';

-- +goose Down
ALTER TABLE settings DROP COLUMN default_client_profile;
