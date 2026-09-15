-- +goose Up
-- The flag that keeps a file out of the candidate list meant "already HEVC".
-- With AV1 as a target it means "already in an efficient codec": AV1 and VVC
-- sources join HEVC, since re-encoding between them loses a generation of
-- quality for little or negative size change.
DROP INDEX IF EXISTS idx_media_files_candidates;
ALTER TABLE media_files RENAME COLUMN is_already_hevc TO is_efficient_codec;
CREATE INDEX IF NOT EXISTS idx_media_files_candidates ON media_files(status, is_efficient_codec)
    WHERE status = 'active' AND is_efficient_codec = 0;

UPDATE media_files
SET is_efficient_codec = 1, predicted_savings_bytes = 0
WHERE is_efficient_codec = 0
  AND LOWER(COALESCE(video_codec, '')) IN ('hevc', 'h265', 'av1', 'vvc', 'h266');

-- library_stats sums predicted_savings_bytes, which the update above just
-- zeroed for AV1 rows. Emptying it makes the boot bootstrap rebuild it from
-- media_files rather than carry the stale totals.
DELETE FROM library_stats;

-- The codec a profile encodes to. Every existing profile was an x265 profile.
ALTER TABLE transcode_profiles ADD COLUMN codec TEXT NOT NULL DEFAULT 'hevc';

-- Snapshot of the codec a job was queued (and, once claimed, encoded) with,
-- alongside the existing preset/CRF snapshot.
ALTER TABLE transcode_jobs ADD COLUMN encode_codec TEXT;
UPDATE transcode_jobs SET encode_codec = 'hevc' WHERE encode_codec IS NULL;

-- Encode rows record their output codec in the column replacements already
-- use, so learned ratios and Insights can split by target.
UPDATE savings_ledger SET result_codec = 'hevc'
WHERE source = 'encode' AND result_codec IS NULL;

-- +goose Down
UPDATE savings_ledger SET result_codec = NULL WHERE source = 'encode';
ALTER TABLE transcode_jobs DROP COLUMN encode_codec;
ALTER TABLE transcode_profiles DROP COLUMN codec;
DROP INDEX IF EXISTS idx_media_files_candidates;
ALTER TABLE media_files RENAME COLUMN is_efficient_codec TO is_already_hevc;
UPDATE media_files SET is_already_hevc = 0
WHERE LOWER(COALESCE(video_codec, '')) NOT IN ('hevc', 'h265');
CREATE INDEX IF NOT EXISTS idx_media_files_candidates ON media_files(status, is_already_hevc)
    WHERE status = 'active' AND is_already_hevc = 0;
DELETE FROM library_stats;
