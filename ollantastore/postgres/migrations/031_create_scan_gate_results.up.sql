-- Immutable snapshot of gate evaluation result per scan.
-- Survives gate edits or deletion for audit trail.
CREATE TABLE IF NOT EXISTS scan_gate_results (
    id           BIGSERIAL PRIMARY KEY,
    scan_id      BIGINT       NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    gate_id      BIGINT       NOT NULL,
    status       TEXT         NOT NULL,
    conditions   JSONB        NOT NULL,
    evaluated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (scan_id)
);
