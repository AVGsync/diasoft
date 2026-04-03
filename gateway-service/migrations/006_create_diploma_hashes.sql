CREATE TABLE diploma_hashes (
    hash           VARCHAR(64) PRIMARY KEY,
    vuz_id         UUID NOT NULL REFERENCES universities(id),
    diploma_number VARCHAR(50) NOT NULL,
    status         VARCHAR(20) NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'revoked')),
    signature      TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at     TIMESTAMPTZ,

    UNIQUE(vuz_id, diploma_number)
);

CREATE INDEX idx_diploma_hashes_vuz_id ON diploma_hashes(vuz_id);
CREATE INDEX idx_diploma_hashes_diploma_number ON diploma_hashes(diploma_number);
