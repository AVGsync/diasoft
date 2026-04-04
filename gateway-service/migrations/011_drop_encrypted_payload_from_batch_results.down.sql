ALTER TABLE batch_results
    ADD COLUMN IF NOT EXISTS encrypted_payload TEXT;
