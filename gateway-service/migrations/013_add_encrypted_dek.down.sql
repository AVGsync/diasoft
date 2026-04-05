-- Remove encrypted_dek column from diploma_hashes table

ALTER TABLE diploma_hashes
    DROP COLUMN IF EXISTS encrypted_dek;
