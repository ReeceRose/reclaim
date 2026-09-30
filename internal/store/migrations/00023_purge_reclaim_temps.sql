-- +goose Up
-- The scanner used to index the worker's own temp outputs: they keep the
-- original's extension (a.avi.reclaim-tmp.avi), and a failed verification left
-- one on disk. Each became a row of its own and a re-encode candidate. The
-- scanner now skips them; this drops the rows it already made, along with any
-- jobs queued against them.
DELETE FROM transcode_jobs
WHERE media_file_id IN (SELECT id FROM media_files WHERE path LIKE '%.reclaim-tmp%');

DELETE FROM media_files WHERE path LIKE '%.reclaim-tmp%';

-- A failed job's output_path pointed at the temp it kept. Failures no longer
-- keep one, and the boot sweep deletes those still beside an original, so the
-- path would only name a file that is gone.
UPDATE transcode_jobs SET output_path = NULL WHERE status = 'failed';

-- The deleted rows contributed to library_stats; emptying it makes the boot
-- bootstrap rebuild it from media_files.
DELETE FROM library_stats;

-- +goose Down
SELECT 1;
