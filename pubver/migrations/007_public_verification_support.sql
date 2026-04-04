ALTER TABLE universities
    ADD COLUMN IF NOT EXISTS vuz_code VARCHAR(50);

CREATE UNIQUE INDEX IF NOT EXISTS ux_universities_vuz_code
    ON universities(vuz_code)
    WHERE vuz_code IS NOT NULL;

-- Public fields year and specialty are read from batch_record_attributes
-- via batch_results.diploma_hash -> (batch_id, record_index).
-- They remain outside universities and diploma_hashes, so this migration
-- still only adds public lookup support through universities.vuz_code.
