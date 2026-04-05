CREATE TABLE IF NOT EXISTS api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vuz_id     UUID NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
    key_hash   VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(100),
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_vuz_id ON api_keys(vuz_id);
