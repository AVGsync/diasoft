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
    u.name
FROM diploma_hashes dh
JOIN universities u ON u.id = dh.vuz_id
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

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&record.Hash,
		&record.DiplomaNumber,
		&status,
		&revokedAt,
		&universityCode,
		&record.University.Name,
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

	return &record, nil
}
