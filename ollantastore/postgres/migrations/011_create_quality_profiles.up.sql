-- Quality profiles: named sets of active rules per language.
CREATE TABLE IF NOT EXISTS quality_profiles (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    language    TEXT        NOT NULL,
    parent_id   BIGINT      REFERENCES quality_profiles(id) ON DELETE SET NULL,
    is_default  BOOLEAN     NOT NULL DEFAULT FALSE,
    is_builtin  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, language)
);

-- Active rules within a profile.
CREATE TABLE IF NOT EXISTS quality_profile_rules (
    id          BIGSERIAL PRIMARY KEY,
    profile_id  BIGINT NOT NULL REFERENCES quality_profiles(id) ON DELETE CASCADE,
    rule_key    TEXT   NOT NULL,
    severity    TEXT   NOT NULL DEFAULT 'major',
    params      JSONB  NOT NULL DEFAULT '{}',
    UNIQUE (profile_id, rule_key)
);

-- Many-to-many: which profile is active for a project+language.
CREATE TABLE IF NOT EXISTS project_profiles (
    project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    language    TEXT   NOT NULL,
    profile_id  BIGINT NOT NULL REFERENCES quality_profiles(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, language)
);

-- Built-in "Ollanta Way" profiles per supported language.
INSERT INTO quality_profiles (name, language, is_default, is_builtin)
VALUES
    ('Ollanta Way', 'go',         TRUE, TRUE),
    ('Ollanta Way', 'java',       TRUE, TRUE),
    ('Ollanta Way', 'python',     TRUE, TRUE),
    ('Ollanta Way', 'javascript', TRUE, TRUE),
    ('Ollanta Way', 'typescript', TRUE, TRUE),
    ('Ollanta Way', 'csharp',     TRUE, TRUE)
ON CONFLICT (name, language) DO NOTHING;
