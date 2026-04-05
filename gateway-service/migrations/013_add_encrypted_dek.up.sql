-- Add encrypted_dek column to diploma_hashes table
-- This column stores the Data Encryption Key (DEK) encrypted with UPEK
-- Part of the key hierarchy refactoring: KEK -> UPEK -> DEK

ALTER TABLE diploma_hashes
    ADD COLUMN IF NOT EXISTS encrypted_dek TEXT;
