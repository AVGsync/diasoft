package postgres

import (
	"context"
	"database/sql"
	"errors"

	"pubver/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const baseVerificationQuery = `
SELECT
    dh.hash,
    dh.diploma_number,
    dh.status,
    dh.revoked_at,
    u.vuz_code,
    u.name,
    bra.year,
    bra.specialty,
    bra.degree,
    bra.faculty
FROM diploma_hashes dh
JOIN universities u ON u.id = dh.vuz_id
LEFT JOIN LATERAL (
    SELECT br.batch_id, br.record_index
    FROM batch_results br
    WHERE br.diploma_hash = dh.hash
    ORDER BY br.created_at DESC
    LIMIT 1
) latest_br ON true
LEFT JOIN batch_record_attributes bra
    ON bra.batch_id = latest_br.batch_id
   AND bra.record_index = latest_br.record_index
`

type VerificationRepository struct {
	pool *pgxpool.Pool
}

func NewVerificationRepository(pool *pgxpool.Pool) *VerificationRepository {
	return &VerificationRepository{pool: pool}
}

func (r *VerificationRepository) FindByHash(ctx context.Context, hash string) (*domain.DiplomaRecord, error) {
	query := baseVerificationQuery + ` WHERE dh.hash = $1 LIMIT 1`
	return r.fetchOne(ctx, query, hash)
}

func (r *VerificationRepository) FindByDiplomaNumber(ctx context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error) {
	query := baseVerificationQuery + ` WHERE u.vuz_code = $1 AND dh.diploma_number = $2 LIMIT 1`
	return r.fetchOne(ctx, query, vuzCode, diplomaNumber)
}

func (r *VerificationRepository) FindUniversityVerificationKeyByID(ctx context.Context, vuzID string) (*domain.UniversityVerificationKey, error) {
	verificationKey := domain.UniversityVerificationKey{}
	var publicKey sql.NullString

	err := r.pool.QueryRow(
		ctx,
		`SELECT public_key FROM universities WHERE id = $1 LIMIT 1`,
		vuzID,
	).Scan(&publicKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUniversityVerificationKeyNotFound
		}

		return nil, err
	}

	if !publicKey.Valid || publicKey.String == "" {
		return nil, domain.ErrUniversityVerificationKeyNotFound
	}

	verificationKey.PublicKey = publicKey.String
	return &verificationKey, nil
}

func (r *VerificationRepository) fetchOne(ctx context.Context, query string, args ...any) (*domain.DiplomaRecord, error) {
	record := domain.DiplomaRecord{}
	var status string
	var revokedAt sql.NullTime
	var universityCode sql.NullString
	var specialty sql.NullString
	var graduateYear sql.NullInt64
	var degree sql.NullString
	var faculty sql.NullString

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&record.Hash,
		&record.DiplomaNumber,
		&status,
		&revokedAt,
		&universityCode,
		&record.University.Name,
		&graduateYear,
		&specialty,
		&degree,
		&faculty,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	record.Status = domain.DiplomaStatus(status)
	record.University.Code = universityCode.String

	if revokedAt.Valid {
		record.RevokedAt = &revokedAt.Time
	}
	if graduateYear.Valid {
		year := int(graduateYear.Int64)
		record.GraduateYear = &year
	}
	if specialty.Valid {
		value := specialty.String
		record.Specialty = &value
	}
	if degree.Valid {
		value := degree.String
		record.Degree = &value
	}
	if faculty.Valid {
		value := faculty.String
		record.Faculty = &value
	}

	return &record, nil
}
