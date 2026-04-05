CREATE TABLE verification_events (
    id             BIGSERIAL PRIMARY KEY,
    event_id       VARCHAR(64) NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_service VARCHAR(50) NOT NULL,
    endpoint       VARCHAR(50) NOT NULL,
    request_id     VARCHAR(64),
    vuz_id         UUID REFERENCES universities(id),
    vuz_code       VARCHAR(20),
    diploma_hash   VARCHAR(64),
    status         VARCHAR(50) NOT NULL,
    is_valid       BOOLEAN NOT NULL DEFAULT FALSE,
    country        VARCHAR(120),
    city           VARCHAR(120),
    client_ip_hash VARCHAR(64),
    user_agent     TEXT
);

CREATE INDEX idx_verification_events_created_at
    ON verification_events(created_at DESC);

CREATE INDEX idx_verification_events_vuz_id_created_at
    ON verification_events(vuz_id, created_at DESC);

CREATE INDEX idx_verification_events_vuz_code_created_at
    ON verification_events(vuz_code, created_at DESC);

CREATE INDEX idx_verification_events_status_created_at
    ON verification_events(status, created_at DESC);

CREATE INDEX idx_verification_events_endpoint_created_at
    ON verification_events(endpoint, created_at DESC);

CREATE INDEX idx_verification_events_country_created_at
    ON verification_events(country, created_at DESC);
