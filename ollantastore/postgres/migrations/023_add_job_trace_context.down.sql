ALTER TABLE webhook_jobs
    DROP COLUMN IF EXISTS trace_state,
    DROP COLUMN IF EXISTS trace_parent;

ALTER TABLE index_jobs
    DROP COLUMN IF EXISTS trace_state,
    DROP COLUMN IF EXISTS trace_parent;

ALTER TABLE scan_jobs
    DROP COLUMN IF EXISTS trace_state,
    DROP COLUMN IF EXISTS trace_parent;