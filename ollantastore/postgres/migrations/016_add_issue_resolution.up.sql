-- Add resolution fields to issues (issues is partitioned; alter the parent).
ALTER TABLE issues ADD COLUMN IF NOT EXISTS resolved_by  BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS assignee_id  BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE issues ADD COLUMN IF NOT EXISTS resolved_at  TIMESTAMPTZ;

-- Transition history for issues.
CREATE TABLE IF NOT EXISTS issue_transitions (
    id          BIGSERIAL PRIMARY KEY,
    issue_id    BIGINT NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_status TEXT   NOT NULL,
    to_status   TEXT   NOT NULL,
    resolution  TEXT   NOT NULL DEFAULT '',
    comment     TEXT   NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_issue_transitions_issue ON issue_transitions (issue_id, created_at DESC);
