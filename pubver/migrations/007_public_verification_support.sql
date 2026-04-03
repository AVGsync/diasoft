ALTER TABLE universities
    ADD COLUMN IF NOT EXISTS vuz_code VARCHAR(50);

CREATE UNIQUE INDEX IF NOT EXISTS ux_universities_vuz_code
    ON universities(vuz_code)
    WHERE vuz_code IS NOT NULL;

-- diploma_publications stores safe public fields for search/verify responses.
-- Populated by Crypto Engine (main API) during batch processing.
-- Until populated, LEFT JOIN yields NULL for graduate_year and specialty.
CREATE TABLE IF NOT EXISTS diploma_publications (
    diploma_hash   VARCHAR(64) PRIMARY KEY REFERENCES diploma_hashes(hash) ON DELETE CASCADE,
    graduate_year  INTEGER NOT NULL CHECK (graduate_year BETWEEN 1900 AND 2100),
    specialty      VARCHAR(255) NOT NULL,
    qualification  VARCHAR(255),
    issued_at      DATE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_diploma_publications_graduate_year
    ON diploma_publications(graduate_year);
