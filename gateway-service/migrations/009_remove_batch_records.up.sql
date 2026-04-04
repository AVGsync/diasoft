ALTER TABLE batch_results DROP CONSTRAINT IF EXISTS batch_results_diploma_hash_fkey;

ALTER TABLE batch_results
    ALTER COLUMN diploma_hash DROP NOT NULL,
    ALTER COLUMN qr_payload DROP NOT NULL;

ALTER TABLE batch_results
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'ok'
        CHECK (status IN ('ok', 'error')),
    ADD COLUMN IF NOT EXISTS error TEXT;

DROP INDEX IF EXISTS idx_batch_records_batch_id;
DROP INDEX IF EXISTS idx_batch_records_diploma_number;
DROP INDEX IF EXISTS idx_batch_records_full_name;

DROP TABLE IF EXISTS batch_records;
