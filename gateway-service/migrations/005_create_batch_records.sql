CREATE TABLE batch_records (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id       UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    record_index   INTEGER NOT NULL,
    full_name      VARCHAR(255) NOT NULL,
    diploma_number VARCHAR(50) NOT NULL,
    specialty      VARCHAR(255) NOT NULL,
    degree         VARCHAR(50) NOT NULL,
    faculty        VARCHAR(255) NOT NULL,
    year           INTEGER NOT NULL,
    raw_csv_row    TEXT,
    status         VARCHAR(20) NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'processed', 'error')),
    error          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(batch_id, record_index)
);

CREATE INDEX idx_batch_records_batch_id ON batch_records(batch_id);
CREATE INDEX idx_batch_records_diploma_number ON batch_records(diploma_number);
CREATE INDEX idx_batch_records_full_name ON batch_records(lower(full_name));
