ALTER TABLE universities
    ADD COLUMN IF NOT EXISTS vuz_code VARCHAR(50);

CREATE UNIQUE INDEX IF NOT EXISTS ux_universities_vuz_code
    ON universities(vuz_code)
    WHERE vuz_code IS NOT NULL;

-- Reserved API placeholders:
-- year and specialty remain in the public contract, but are intentionally
-- not added to PostgreSQL yet while the storage model is still being finalized.
-- Public Verification API returns them as null placeholders for now.
