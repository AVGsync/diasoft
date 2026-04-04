CREATE TABLE universities (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vuz_code      VARCHAR(20) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    inn           VARCHAR(12) NOT NULL UNIQUE,
    ogrn          VARCHAR(15) NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    public_key    TEXT,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'active', 'blocked')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
