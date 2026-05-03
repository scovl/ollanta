DROP INDEX IF EXISTS idx_webhook_jobs_status_updated_at;
DROP INDEX IF EXISTS idx_index_jobs_status_updated_at;
DROP INDEX IF EXISTS idx_scan_jobs_status_updated_at;
DROP INDEX IF EXISTS idx_webhook_jobs_active_delivery;
DROP INDEX IF EXISTS idx_index_jobs_active_scan;
DROP INDEX IF EXISTS idx_scan_jobs_idempotency_identity;

ALTER TABLE webhook_jobs
    DROP COLUMN IF EXISTS payload_hash;

ALTER TABLE scan_jobs
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS payload_hash,
    DROP COLUMN IF EXISTS idempotency_key;
