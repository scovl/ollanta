-- Persistent quality gates.
CREATE TABLE IF NOT EXISTS quality_gates (
    id                   BIGSERIAL PRIMARY KEY,
    name                 TEXT    NOT NULL UNIQUE,
    is_default           BOOLEAN NOT NULL DEFAULT FALSE,
    is_builtin           BOOLEAN NOT NULL DEFAULT FALSE,
    small_changeset_lines INT    NOT NULL DEFAULT 20,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Conditions attached to a gate.
CREATE TABLE IF NOT EXISTS gate_conditions (
    id          BIGSERIAL PRIMARY KEY,
    gate_id     BIGINT  NOT NULL REFERENCES quality_gates(id) ON DELETE CASCADE,
    metric      TEXT    NOT NULL,
    operator    TEXT    NOT NULL, -- GT, LT, GTE, LTE, EQ, NE
    threshold   NUMERIC NOT NULL,
    on_new_code BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (gate_id, metric, on_new_code)
);

-- Many-to-one: which gate is active for a project.
CREATE TABLE IF NOT EXISTS project_gates (
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    gate_id    BIGINT NOT NULL REFERENCES quality_gates(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id)
);

-- Built-in "Ollanta Default" gate.
INSERT INTO quality_gates (name, is_default, is_builtin)
VALUES ('Ollanta Default', TRUE, TRUE)
ON CONFLICT (name) DO NOTHING;

-- Default conditions on the built-in gate.
-- Condition IDs are stable via named CTE to avoid hard-coding BIGSERIAL values.
WITH g AS (SELECT id FROM quality_gates WHERE name = 'Ollanta Default')
INSERT INTO gate_conditions (gate_id, metric, operator, threshold, on_new_code)
SELECT g.id, m.metric, m.operator, m.threshold, m.on_new_code
FROM g, (VALUES
    ('bugs',            'GT', 0, FALSE),
    ('vulnerabilities', 'GT', 0, FALSE),
    ('new_bugs',        'GT', 0, TRUE),
    ('new_vulnerabilities', 'GT', 0, TRUE)
) AS m(metric, operator, threshold, on_new_code)
ON CONFLICT (gate_id, metric, on_new_code) DO NOTHING;
