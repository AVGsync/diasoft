CREATE TABLE IF NOT EXISTS batch_record_payloads (
    batch_id           UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    record_index       INTEGER NOT NULL,
    encrypted_payload  TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (batch_id, record_index)
);

CREATE INDEX IF NOT EXISTS idx_batch_record_payloads_batch_id
    ON batch_record_payloads(batch_id);

DROP INDEX IF EXISTS idx_batch_record_attributes_batch_id;
DROP TABLE IF EXISTS batch_record_attributes;
