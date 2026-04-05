CREATE TABLE IF NOT EXISTS batch_results (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id          UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    record_index      INTEGER NOT NULL,
    diploma_hash      VARCHAR(64),
    qr_payload        TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'ok'
                      CHECK (status IN ('ok', 'error')),
    error             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(batch_id, record_index),
    UNIQUE(diploma_hash)
);

CREATE INDEX IF NOT EXISTS idx_batch_results_batch_id ON batch_results(batch_id);
CREATE INDEX IF NOT EXISTS idx_batch_results_diploma_hash ON batch_results(diploma_hash);
