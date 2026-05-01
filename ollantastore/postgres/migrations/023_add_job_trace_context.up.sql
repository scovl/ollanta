ALTER TABLE scan_jobs
    ADD COLUMN IF NOT EXISTS trace_parent TEXT,
    ADD COLUMN IF NOT EXISTS trace_state TEXT;

ALTER TABLE index_jobs
    ADD COLUMN IF NOT EXISTS trace_parent TEXT,
    ADD COLUMN IF NOT EXISTS trace_state TEXT;

ALTER TABLE webhook_jobs
    ADD COLUMN IF NOT EXISTS trace_parent TEXT,
    ADD COLUMN IF NOT EXISTS trace_state TEXT;