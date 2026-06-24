-- +goose Up
-- library_stats is a small denormalized aggregates table (§12) maintained
-- incrementally on every media_files change so the overview never has to run a
-- GROUP BY over the whole library on dashboard open.
--
-- Rows are bucketed by (dimension, bucket):
--   dimension='total'      bucket=''            → whole-library totals
--   dimension='codec'      bucket=<video_codec> → per source codec ('unknown' if null)
--   dimension='resolution' bucket=<band>        → 'sd' | 'hd' | 'uhd' | 'unknown'
-- Only active files contribute. A bucket is deleted when its file_count hits 0,
-- so the table matches a from-scratch recompute exactly.
CREATE TABLE IF NOT EXISTS library_stats (
    dimension TEXT NOT NULL,
    bucket TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    predicted_savings_bytes INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (dimension, bucket)
);

-- +goose Down
DROP TABLE IF EXISTS library_stats;
