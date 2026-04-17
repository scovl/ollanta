-- New code period settings (branch > project > global precedence).
CREATE TABLE IF NOT EXISTS new_code_periods (
    id          BIGSERIAL PRIMARY KEY,
    -- scope: 'global', 'project', or 'branch'
    scope       TEXT   NOT NULL DEFAULT 'global',
    project_id  BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    branch      TEXT,
    -- strategy: previous_version | number_of_days | specific_analysis | reference_branch | auto
    strategy    TEXT   NOT NULL DEFAULT 'auto',
    value       TEXT   NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scope, project_id, branch)
);

-- Global default: auto strategy.
INSERT INTO new_code_periods (scope, strategy, value)
VALUES ('global', 'auto', '')
ON CONFLICT (scope, project_id, branch) DO NOTHING;
