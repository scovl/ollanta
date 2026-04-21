CREATE TABLE scan_jobs (
    id BIGSERIAL PRIMARY KEY,
    project_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'running', 'completed', 'failed')),
    payload JSONB NOT NULL,
    scan_id BIGINT REFERENCES scans(id) ON DELETE SET NULL,
    worker_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_scan_jobs_status_created_at ON scan_jobs (status, created_at, id);
CREATE INDEX idx_scan_jobs_project_key ON scan_jobs (project_key);