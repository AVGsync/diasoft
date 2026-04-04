CREATE TABLE university_signing_keys (
    vuz_id                 UUID PRIMARY KEY REFERENCES universities(id) ON DELETE CASCADE,
    encrypted_private_key  TEXT NOT NULL,
    key_algorithm          VARCHAR(20) NOT NULL DEFAULT 'ed25519'
                           CHECK (key_algorithm IN ('ed25519')),
    encryption_algorithm   VARCHAR(50) NOT NULL DEFAULT 'aes-256-gcm',
    public_key_fingerprint VARCHAR(64) NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_university_signing_keys_public_key_fingerprint
    ON university_signing_keys(public_key_fingerprint);
