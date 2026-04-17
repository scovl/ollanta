-- Webhook endpoints registered by users.
CREATE TABLE IF NOT EXISTS webhooks (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT  REFERENCES projects(id) ON DELETE CASCADE, -- NULL = global
    name        TEXT    NOT NULL,
    url         TEXT    NOT NULL,
    secret      TEXT    NOT NULL DEFAULT '',
    events      TEXT[]  NOT NULL DEFAULT '{}', -- empty = all events
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Record of each delivery attempt.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id            BIGSERIAL PRIMARY KEY,
    webhook_id    BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event         TEXT   NOT NULL,
    payload       JSONB  NOT NULL DEFAULT '{}',
    response_code INT,
    response_body TEXT,
    success       BOOLEAN NOT NULL DEFAULT FALSE,
    attempt       INT     NOT NULL DEFAULT 1,
    delivered_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, delivered_at DESC);
