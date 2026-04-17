DROP TABLE IF EXISTS issue_transitions;
ALTER TABLE issues DROP COLUMN IF EXISTS resolved_at;
ALTER TABLE issues DROP COLUMN IF EXISTS assignee_id;
ALTER TABLE issues DROP COLUMN IF EXISTS resolved_by;
