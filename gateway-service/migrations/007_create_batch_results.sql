CREATE TABLE batch_results (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id          UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    record_index      INTEGER NOT NULL,
    diploma_hash      VARCHAR(64) NOT NULL REFERENCES diploma_hashes(hash),
    encrypted_payload TEXT NOT NULL,
    qr_payload        TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(batch_id, record_index),
    UNIQUE(diploma_hash)
);

CREATE INDEX idx_batch_results_batch_id ON batch_results(batch_id);
CREATE INDEX idx_batch_results_diploma_hash ON batch_results(diploma_hash);
