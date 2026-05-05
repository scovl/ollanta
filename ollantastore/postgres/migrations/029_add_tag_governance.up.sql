CREATE TABLE IF NOT EXISTS tag_catalog (
    id              BIGSERIAL PRIMARY KEY,
    key             TEXT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    color           TEXT NOT NULL DEFAULT '',
    owner_type      TEXT NOT NULL DEFAULT '',
    owner_id        BIGINT NOT NULL DEFAULT 0,
    owner_name      TEXT NOT NULL DEFAULT '',
    scope           TEXT NOT NULL DEFAULT 'global',
    status          TEXT NOT NULL DEFAULT 'active',
    source          TEXT NOT NULL DEFAULT 'manual',
    replacement_key TEXT NOT NULL DEFAULT '',
    created_by      BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tag_catalog_status ON tag_catalog (status);
CREATE INDEX IF NOT EXISTS idx_tag_catalog_owner ON tag_catalog (owner_type, owner_id, owner_name);
CREATE INDEX IF NOT EXISTS idx_tag_catalog_search ON tag_catalog USING gin (to_tsvector('simple', key || ' ' || display_name || ' ' || description || ' ' || owner_name));

CREATE TABLE IF NOT EXISTS tag_aliases (
    id         BIGSERIAL PRIMARY KEY,
    tag_key    TEXT NOT NULL REFERENCES tag_catalog(key) ON DELETE CASCADE,
    alias      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (alias <> tag_key)
);

CREATE INDEX IF NOT EXISTS idx_tag_aliases_tag ON tag_aliases (tag_key);

CREATE TABLE IF NOT EXISTS tag_assignments (
    id            BIGSERIAL PRIMARY KEY,
    target_type   TEXT NOT NULL,
    target_id     BIGINT NOT NULL DEFAULT 0,
    target_key    TEXT NOT NULL DEFAULT '',
    tag_key       TEXT NOT NULL REFERENCES tag_catalog(key) ON DELETE CASCADE,
    source        TEXT NOT NULL DEFAULT 'manual',
    actor_user_id BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (target_type, target_id, target_key, tag_key)
);

CREATE INDEX IF NOT EXISTS idx_tag_assignments_tag ON tag_assignments (tag_key);
CREATE INDEX IF NOT EXISTS idx_tag_assignments_target ON tag_assignments (target_type, target_id, target_key);

CREATE TABLE IF NOT EXISTS saved_filters (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    owner_user_id BIGINT NOT NULL DEFAULT 0,
    visibility    TEXT NOT NULL DEFAULT 'private',
    filter_type   TEXT NOT NULL DEFAULT 'issues',
    criteria      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_saved_filters_owner ON saved_filters (owner_user_id, visibility, filter_type);
CREATE INDEX IF NOT EXISTS idx_saved_filters_criteria ON saved_filters USING gin (criteria);

CREATE TABLE IF NOT EXISTS tag_audit (
    id            BIGSERIAL PRIMARY KEY,
    tag_key       TEXT NOT NULL DEFAULT '',
    action        TEXT NOT NULL,
    target_type   TEXT NOT NULL DEFAULT '',
    target_id     BIGINT NOT NULL DEFAULT 0,
    target_key    TEXT NOT NULL DEFAULT '',
    actor_user_id BIGINT NOT NULL DEFAULT 0,
    old_state     JSONB NOT NULL DEFAULT '{}',
    new_state     JSONB NOT NULL DEFAULT '{}',
    summary       JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tag_audit_tag_created ON tag_audit (tag_key, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tag_audit_target_created ON tag_audit (target_type, target_id, target_key, created_at DESC);

CREATE TABLE IF NOT EXISTS tag_vocabulary_policies (
    project_id  BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    restricted  BOOLEAN NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

WITH existing_tags AS (
    SELECT DISTINCT regexp_replace(lower(btrim(tag)), '[[:space:]]+', '-', 'g') AS key, btrim(tag) AS display_name
    FROM (
        SELECT unnest(tags) AS tag FROM projects
        UNION ALL
        SELECT unnest(tags) AS tag FROM issues
        UNION ALL
        SELECT unnest(tags) AS tag FROM custom_rule_versions
    ) raw
    WHERE btrim(tag) <> ''
)
INSERT INTO tag_catalog (key, display_name, status, source)
SELECT key, display_name, 'discovered', 'backfill'
FROM existing_tags
WHERE key <> ''
    AND key ~ '^[a-z0-9][a-z0-9._:-]{0,62}$'
    AND key NOT IN ('all', 'none', 'null', 'undefined')
ON CONFLICT (key) DO NOTHING;
