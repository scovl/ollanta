ALTER TABLE scan_jobs DROP CONSTRAINT IF EXISTS scan_jobs_status_check;
ALTER TABLE scan_jobs ADD CONSTRAINT scan_jobs_status_check CHECK (status IN ('accepted', 'running', 'completed', 'failed', 'cancelled'));

ALTER TABLE index_jobs DROP CONSTRAINT IF EXISTS index_jobs_status_check;
ALTER TABLE index_jobs ADD CONSTRAINT index_jobs_status_check CHECK (status IN ('accepted', 'running', 'completed', 'failed', 'cancelled'));

ALTER TABLE webhook_jobs DROP CONSTRAINT IF EXISTS webhook_jobs_status_check;
ALTER TABLE webhook_jobs ADD CONSTRAINT webhook_jobs_status_check CHECK (status IN ('accepted', 'running', 'completed', 'failed', 'cancelled'));
