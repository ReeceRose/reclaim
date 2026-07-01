-- +goose Up
-- Per-file, per-client-profile compatibility verdicts (internal/compatibility).
-- Computed at scan time for every built-in profile and stored here so list
-- pagination/stats never re-evaluate rules per request — see
-- docs/COMPATIBILITY PLAN.md §6 "Scoring at scan time vs query time".
CREATE TABLE IF NOT EXISTS media_compatibility (
    media_file_id INTEGER NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    client_profile TEXT NOT NULL,
    risk_score INTEGER NOT NULL,
    direct_play_predicted INTEGER NOT NULL,
    reasons_json TEXT NOT NULL,
    recommended_action TEXT NOT NULL,
    evaluated_at INTEGER NOT NULL,
    PRIMARY KEY (media_file_id, client_profile)
);
CREATE INDEX IF NOT EXISTS idx_compatibility_risk ON media_compatibility(client_profile, risk_score DESC, media_file_id ASC);

-- +goose Down
DROP INDEX IF EXISTS idx_compatibility_risk;
DROP TABLE IF EXISTS media_compatibility;
