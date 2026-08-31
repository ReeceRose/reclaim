-- +goose Up
-- Replacement rows that reclaimed nothing and cost nothing. Most are one file
-- returning rather than being replaced: a redownload of the same release, or a
-- copy restored from a backup, landing under a new name while the original row
-- sat missing. The scanner now recognises those by fingerprint and folds them
-- as a move, but the rows already booked would keep inflating the replacement
-- count against a net of zero, so they go.
DELETE FROM savings_ledger
WHERE source = 'replace' AND original_size_bytes = output_size_bytes;

-- +goose Down
-- The rows carried no bytes, so there is nothing to restore.
SELECT 1;
