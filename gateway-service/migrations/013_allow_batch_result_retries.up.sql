ALTER TABLE batch_results
    DROP CONSTRAINT IF EXISTS batch_results_diploma_hash_key;

CREATE UNIQUE INDEX IF NOT EXISTS ux_universities_vuz_code
    ON universities(vuz_code)
    WHERE vuz_code IS NOT NULL;
