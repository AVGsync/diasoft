CREATE TABLE batch_record_attributes (
    batch_id     UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    record_index INTEGER NOT NULL,
    specialty    VARCHAR(255) NOT NULL,
    degree       VARCHAR(50) NOT NULL,
    faculty      VARCHAR(255) NOT NULL,
    year         INTEGER NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (batch_id, record_index)
);

CREATE INDEX idx_batch_record_attributes_batch_id ON batch_record_attributes(batch_id);
