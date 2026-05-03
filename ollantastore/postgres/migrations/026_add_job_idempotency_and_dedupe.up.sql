ALTER TABLE scan_jobs
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payload_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE webhook_jobs
    ADD COLUMN IF NOT EXISTS payload_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_jobs_idempotency_identity
    ON scan_jobs (project_key, idempotency_key)
    WHERE idempotency_key <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_index_jobs_active_scan
    ON index_jobs (scan_id)
    WHERE status IN ('accepted', 'running');

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_jobs_active_delivery
    ON webhook_jobs (webhook_id, event, payload_hash)
    WHERE status IN ('accepted', 'running') AND payload_hash <> '';

CREATE INDEX IF NOT EXISTS idx_scan_jobs_status_updated_at
    ON scan_jobs (status, updated_at, id);

CREATE INDEX IF NOT EXISTS idx_index_jobs_status_updated_at
    ON index_jobs (status, updated_at, id);

CREATE INDEX IF NOT EXISTS idx_webhook_jobs_status_updated_at
    ON webhook_jobs (status, updated_at, id);
