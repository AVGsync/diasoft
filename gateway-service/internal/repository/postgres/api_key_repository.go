package postgres

import (
	"context"
	"database/sql"

	"github.com/diasoft/gateway-service/internal/model"
)

type APIKeyRepository struct {
	database *DB
}

func (r *APIKeyRepository) Create(ctx context.Context, vuzID string, name *string, keyHash string) (*model.APIKey, error) {
	apiKey := &model.APIKey{}
	var nullableName sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`INSERT INTO api_keys (vuz_id, key_hash, name)
		 VALUES ($1, $2, $3)
		 RETURNING id, vuz_id, key_hash, name, is_active, created_at`,
		vuzID,
		keyHash,
		name,
	).Scan(
		&apiKey.ID,
		&apiKey.VUZID,
		&apiKey.KeyHash,
		&nullableName,
		&apiKey.IsActive,
		&apiKey.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if nullableName.Valid {
		apiKey.Name = &nullableName.String
	}

	return apiKey, nil
}

func (r *APIKeyRepository) ListActiveByVUZ(ctx context.Context, vuzID string) ([]*model.APIKeySummary, error) {
	rows, err := r.database.db.QueryContext(
		ctx,
		`SELECT id, name, is_active, created_at
		 FROM api_keys
		 WHERE vuz_id = $1 AND is_active = TRUE
		 ORDER BY created_at DESC`,
		vuzID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.APIKeySummary, 0)
	for rows.Next() {
		var item model.APIKeySummary
		var nullableName sql.NullString

		if err := rows.Scan(&item.ID, &nullableName, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}

		if nullableName.Valid {
			item.Name = &nullableName.String
		}

		result = append(result, &item)
	}

	return result, rows.Err()
}

func (r *APIKeyRepository) FindActiveUniversityByHash(ctx context.Context, keyHash string) (*model.University, *model.APIKey, error) {
	university := &model.University{}
	apiKey := &model.APIKey{}
	var publicKey sql.NullString
	var keyName sql.NullString

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT
			u.id, u.name, u.inn, u.ogrn, u.email, u.password_hash, u.public_key, u.status, u.created_at,
			ak.id, ak.vuz_id, ak.key_hash, ak.name, ak.is_active, ak.created_at
		 FROM api_keys ak
		 JOIN universities u ON u.id = ak.vuz_id
		 WHERE ak.key_hash = $1
		   AND ak.is_active = TRUE
		   AND u.status = 'active'`,
		keyHash,
	).Scan(
		&university.ID,
		&university.Name,
		&university.INN,
		&university.OGRN,
		&university.Email,
		&university.PasswordHash,
		&publicKey,
		&university.Status,
		&university.CreatedAt,
		&apiKey.ID,
		&apiKey.VUZID,
		&apiKey.KeyHash,
		&keyName,
		&apiKey.IsActive,
		&apiKey.CreatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	if publicKey.Valid {
		university.PublicKey = &publicKey.String
	}
	if keyName.Valid {
		apiKey.Name = &keyName.String
	}

	return university, apiKey, nil
}
