CREATE TABLE share_links (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    diploma_hash VARCHAR(64) NOT NULL REFERENCES diploma_hashes(hash),
    token        TEXT NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    used_count   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_share_links_diploma_hash ON share_links(diploma_hash);
CREATE INDEX idx_share_links_expires_at ON share_links(expires_at);
