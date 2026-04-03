CREATE TABLE batches (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vuz_id            UUID NOT NULL REFERENCES universities(id),
    status            VARCHAR(20) NOT NULL DEFAULT 'processing'
                      CHECK (status IN ('processing', 'completed', 'failed')),
    total_records     INTEGER NOT NULL DEFAULT 0,
    processed_records INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX idx_batches_vuz_id ON batches(vuz_id);
