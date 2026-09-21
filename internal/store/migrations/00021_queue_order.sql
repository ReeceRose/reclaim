-- +goose Up
-- +goose StatementBegin
-- The queue used to run strictly in queued_at order, which left no way to
-- prioritise one job over another. queue_order is the explicit position: new
-- jobs take MAX+1, and "move to top/bottom" rewrites it past either end.
-- queued_at is left alone so it still records when the job was queued.
ALTER TABLE transcode_jobs ADD COLUMN queue_order INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE transcode_jobs SET queue_order = ranked.rn
FROM (SELECT id, ROW_NUMBER() OVER (ORDER BY queued_at, id) AS rn FROM transcode_jobs) AS ranked
WHERE ranked.id = transcode_jobs.id;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_transcode_jobs_queue_order ON transcode_jobs(status, queue_order, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_transcode_jobs_queue_order;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE transcode_jobs DROP COLUMN queue_order;
-- +goose StatementEnd
