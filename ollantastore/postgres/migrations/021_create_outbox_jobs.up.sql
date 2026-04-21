CREATE TABLE index_jobs (
    id BIGSERIAL PRIMARY KEY,
    scan_id BIGINT NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    project_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'running', 'completed', 'failed')),
    worker_id TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_index_jobs_status_next_attempt ON index_jobs (status, next_attempt_at, id);
CREATE INDEX idx_index_jobs_scan_id ON index_jobs (scan_id);

CREATE TABLE webhook_jobs (
    id BIGSERIAL PRIMARY KEY,
    webhook_id BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    project_id BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    event TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'running', 'completed', 'failed')),
    worker_id TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    last_response_code INTEGER,
    last_response_body TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_jobs_status_next_attempt ON webhook_jobs (status, next_attempt_at, id);
CREATE INDEX idx_webhook_jobs_webhook_id ON webhook_jobs (webhook_id);