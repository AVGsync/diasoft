package postgres

import (
	"context"

	"github.com/diasoft/gateway-service/internal/model"
)

type SigningKeyRepository struct {
	database *DB
}

func (r *SigningKeyRepository) Upsert(ctx context.Context, vuzID, encryptedPrivateKey, keyAlgorithm, encryptionAlgorithm, publicKeyFingerprint string) (*model.UniversitySigningKey, error) {
	item := &model.UniversitySigningKey{}

	err := r.database.db.QueryRowContext(
		ctx,
		`INSERT INTO university_signing_keys
		 (vuz_id, encrypted_private_key, key_algorithm, encryption_algorithm, public_key_fingerprint)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (vuz_id)
		 DO UPDATE SET
			encrypted_private_key = EXCLUDED.encrypted_private_key,
			key_algorithm = EXCLUDED.key_algorithm,
			encryption_algorithm = EXCLUDED.encryption_algorithm,
			public_key_fingerprint = EXCLUDED.public_key_fingerprint,
			updated_at = NOW()
		 RETURNING vuz_id, encrypted_private_key, key_algorithm, encryption_algorithm, public_key_fingerprint, created_at, updated_at`,
		vuzID,
		encryptedPrivateKey,
		keyAlgorithm,
		encryptionAlgorithm,
		publicKeyFingerprint,
	).Scan(
		&item.VUZID,
		&item.EncryptedPrivateKey,
		&item.KeyAlgorithm,
		&item.EncryptionAlgorithm,
		&item.PublicKeyFingerprint,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (r *SigningKeyRepository) FindByVUZID(ctx context.Context, vuzID string) (*model.UniversitySigningKey, error) {
	item := &model.UniversitySigningKey{}

	err := r.database.db.QueryRowContext(
		ctx,
		`SELECT vuz_id, encrypted_private_key, key_algorithm, encryption_algorithm, public_key_fingerprint, created_at, updated_at
		 FROM university_signing_keys
		 WHERE vuz_id = $1`,
		vuzID,
	).Scan(
		&item.VUZID,
		&item.EncryptedPrivateKey,
		&item.KeyAlgorithm,
		&item.EncryptionAlgorithm,
		&item.PublicKeyFingerprint,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return item, nil
}
