-- +goose Up
-- Generic catch-all for rarely-needed, format-level ffprobe fields
-- (format_long_name, nb_streams, probe_score, format tags like
-- encoder/creation_time/title). Mirrors media_streams.extra_json's pattern:
-- capture broadly now so a future feature that wants one of these fields can
-- mine already-stored data instead of forcing every user through another
-- full library rescan (see internal/ffprobe.Result.FormatExtraJSON and
-- docs/COMPATIBILITY PLAN.md §5 "Backfill").
ALTER TABLE media_files ADD COLUMN probe_extra_json TEXT;

-- Promoted to real columns, unlike the broader side_data_list dump kept in
-- media_streams.extra_json: docs/COMPATIBILITY PLAN.md §6 already
-- anticipates a Dolby Vision profile rule (Apple TV 4K: "Profile 5 only",
-- per Apple's tech specs), so this has a concrete near-term consumer rather
-- than being speculative capture.
ALTER TABLE media_files ADD COLUMN dolby_vision_profile INTEGER;
ALTER TABLE media_files ADD COLUMN dolby_vision_level INTEGER;

-- +goose Down
ALTER TABLE media_files DROP COLUMN dolby_vision_level;
ALTER TABLE media_files DROP COLUMN dolby_vision_profile;
ALTER TABLE media_files DROP COLUMN probe_extra_json;
