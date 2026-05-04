CREATE TABLE IF NOT EXISTS custom_rule_packs (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    namespace   TEXT NOT NULL DEFAULT 'custom',
    description TEXT NOT NULL DEFAULT '',
    source_hash TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, namespace)
);

ALTER TABLE scan_profile_snapshots
    ADD COLUMN IF NOT EXISTS custom_catalog_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS custom_rule_versions (
    id                     BIGSERIAL PRIMARY KEY,
    pack_id                BIGINT NOT NULL REFERENCES custom_rule_packs(id) ON DELETE CASCADE,
    rule_key               TEXT NOT NULL,
    version                INT NOT NULL,
    lifecycle              TEXT NOT NULL DEFAULT 'draft',
    name                   TEXT NOT NULL,
    description            TEXT NOT NULL DEFAULT '',
    language               TEXT NOT NULL,
    type                   TEXT NOT NULL,
    severity               TEXT NOT NULL,
    tags                   TEXT[] NOT NULL DEFAULT '{}',
    params_schema          JSONB NOT NULL DEFAULT '{}',
    engine                 TEXT NOT NULL,
    engine_config          JSONB NOT NULL DEFAULT '{}',
    message                TEXT NOT NULL DEFAULT '',
    examples               JSONB NOT NULL DEFAULT '[]',
    limits                 JSONB NOT NULL DEFAULT '{}',
    version_hash           TEXT NOT NULL,
    validation_status      TEXT NOT NULL DEFAULT 'none',
    validation_hash        TEXT NOT NULL DEFAULT '',
    validation_diagnostics JSONB NOT NULL DEFAULT '[]',
    validator_capabilities TEXT[] NOT NULL DEFAULT '{}',
    validation_timestamp   TIMESTAMPTZ,
    published_at           TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rule_key, version)
);

CREATE INDEX IF NOT EXISTS idx_custom_rule_versions_pack
    ON custom_rule_versions(pack_id, rule_key, version DESC);
CREATE INDEX IF NOT EXISTS idx_custom_rule_versions_published
    ON custom_rule_versions(rule_key)
    WHERE lifecycle = 'published';

CREATE TABLE IF NOT EXISTS custom_rule_audit (
    id           BIGSERIAL PRIMARY KEY,
    pack_id      BIGINT REFERENCES custom_rule_packs(id) ON DELETE SET NULL,
    rule_id      BIGINT REFERENCES custom_rule_versions(id) ON DELETE SET NULL,
    rule_key     TEXT NOT NULL DEFAULT '',
    version_hash TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL,
    old_state    TEXT NOT NULL DEFAULT '',
    new_state    TEXT NOT NULL DEFAULT '',
    actor_user_id BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_custom_rule_audit_rule_created
    ON custom_rule_audit(rule_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_custom_rule_audit_pack_created
    ON custom_rule_audit(pack_id, created_at DESC);
