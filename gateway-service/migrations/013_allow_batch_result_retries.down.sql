DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'batch_results_diploma_hash_key'
    ) THEN
        ALTER TABLE batch_results
            ADD CONSTRAINT batch_results_diploma_hash_key UNIQUE (diploma_hash);
    END IF;
END
$$;

DROP INDEX IF EXISTS ux_universities_vuz_code;
