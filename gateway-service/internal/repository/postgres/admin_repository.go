package postgres

import (
	"context"

	"github.com/diasoft/gateway-service/internal/model"
)

type AdminRepository struct {
	database *DB
}

func (r *AdminRepository) UpsertBootstrap(ctx context.Context, email, passwordHash string) error {
	_, err := r.database.db.ExecContext(
		ctx,
		`INSERT INTO platform_admins (email, password_hash)
		 VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
		email,
		passwordHash,
	)
	return err
}

func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*model.PlatformAdmin, error) {
	admin := &model.PlatformAdmin{}
	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT id, email, password_hash, created_at
		 FROM platform_admins
		 WHERE email = $1`,
		email,
	).Scan(&admin.ID, &admin.Email, &admin.PasswordHash, &admin.CreatedAt)
	if err != nil {
		return nil, err
	}

	return admin, nil
}

func (r *AdminRepository) Stats(ctx context.Context) (*model.AdminStats, error) {
	stats := &model.AdminStats{}
	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			(SELECT COUNT(*) FROM universities),
			(SELECT COUNT(*) FROM universities WHERE status = 'pending'),
			(SELECT COUNT(*) FROM universities WHERE status = 'active'),
			(SELECT COUNT(*) FROM universities WHERE status = 'blocked'),
			(SELECT COUNT(*) FROM batches),
			(SELECT COUNT(*) FROM diploma_hashes),
			(SELECT COUNT(*) FROM diploma_hashes WHERE status = 'revoked')`,
	).Scan(
		&stats.TotalUniversities,
		&stats.PendingUniversities,
		&stats.ActiveUniversities,
		&stats.BlockedUniversities,
		&stats.TotalBatches,
		&stats.TotalDiplomas,
		&stats.RevokedDiplomas,
	)
	if err != nil {
		return nil, err
	}

	return stats, nil
}
