CREATE TABLE IF NOT EXISTS batch_records (
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

CREATE INDEX IF NOT EXISTS idx_batch_records_batch_id ON batch_records(batch_id);
CREATE INDEX IF NOT EXISTS idx_batch_records_diploma_number ON batch_records(diploma_number);
CREATE INDEX IF NOT EXISTS idx_batch_records_full_name ON batch_records(lower(full_name));

ALTER TABLE batch_results DROP COLUMN IF EXISTS error;
ALTER TABLE batch_results DROP COLUMN IF EXISTS status;

ALTER TABLE batch_results
    ALTER COLUMN diploma_hash SET NOT NULL,
    ALTER COLUMN qr_payload SET NOT NULL;

ALTER TABLE batch_results
    ADD CONSTRAINT batch_results_diploma_hash_fkey
    FOREIGN KEY (diploma_hash) REFERENCES diploma_hashes(hash);
