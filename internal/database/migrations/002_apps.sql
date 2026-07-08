CREATE TABLE IF NOT EXISTS apps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'created',
    port            INTEGER,
    binary_path     TEXT,
    binary_version  TEXT,
    binary_checksum TEXT,
    env_vars        JSONB NOT NULL DEFAULT '{}'::jsonb,
    custom_domain   TEXT,
    production_vid  UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_apps_user ON apps(user_id);

CREATE TABLE IF NOT EXISTS deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    binary_path     TEXT NOT NULL,
    binary_size     BIGINT,
    checksum        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'uploaded',
    port            INTEGER,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_deployments_app ON deployments(app_id);
