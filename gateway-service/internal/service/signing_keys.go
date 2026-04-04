package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"strings"

	"github.com/diasoft/gateway-service/internal/infrastructure/security"
	"github.com/diasoft/gateway-service/internal/model"
)

const (
	SigningKeyAlgorithmEd25519 = "ed25519"
	SigningKeyEncryptionAESGCM = "aes-256-gcm"
)

type SigningKeyUniversityRepository interface {
	FindByID(ctx context.Context, id string) (*model.University, error)
	UpdatePublicKey(ctx context.Context, id, publicKey string) error
}

type SigningKeyRepository interface {
	Upsert(ctx context.Context, vuzID, encryptedPrivateKey, keyAlgorithm, encryptionAlgorithm, publicKeyFingerprint string) (*model.UniversitySigningKey, error)
	FindByVUZID(ctx context.Context, vuzID string) (*model.UniversitySigningKey, error)
}

type KeyEncryptor interface {
	Seal(plaintext []byte) (string, error)
}

type SigningKeyService struct {
	universities SigningKeyUniversityRepository
	keys         SigningKeyRepository
	encryptor    KeyEncryptor
}

func NewSigningKeyService(universities SigningKeyUniversityRepository, keys SigningKeyRepository, encryptor KeyEncryptor) *SigningKeyService {
	return &SigningKeyService{
		universities: universities,
		keys:         keys,
		encryptor:    encryptor,
	}
}

func (s *SigningKeyService) Upsert(ctx context.Context, vuzID string, request *model.UpsertSigningKeyRequest) (*model.SigningKeyStatusResponse, error) {
	privateKey, err := security.ParseEd25519PrivateKeyPEM(request.PrivateKeyPEM)
	if err != nil {
		return nil, ErrInvalidSigningKey
	}

	derivedPublic, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, ErrInvalidSigningKey
	}

	derivedPublicPEM, err := security.MarshalEd25519PublicKeyPEM(derivedPublic)
	if err != nil {
		return nil, err
	}

	university, err := s.universities.FindByID(ctx, vuzID)
	if err != nil {
		return nil, err
	}

	if university.PublicKey != nil && strings.TrimSpace(*university.PublicKey) != "" {
		storedPublic, err := security.ParseEd25519PublicKeyPEM(*university.PublicKey)
		if err != nil {
			return nil, ErrSigningKeyMismatch
		}
		if !bytes.Equal(storedPublic, derivedPublic) {
			return nil, ErrSigningKeyMismatch
		}
	} else {
		if err := s.universities.UpdatePublicKey(ctx, vuzID, derivedPublicPEM); err != nil {
			return nil, err
		}
	}

	encryptedPrivateKey, err := s.encryptor.Seal([]byte(request.PrivateKeyPEM))
	if err != nil {
		return nil, err
	}

	item, err := s.keys.Upsert(
		ctx,
		vuzID,
		encryptedPrivateKey,
		SigningKeyAlgorithmEd25519,
		SigningKeyEncryptionAESGCM,
		security.FingerprintEd25519PublicKey(derivedPublic),
	)
	if err != nil {
		return nil, err
	}

	return toSigningKeyStatusResponse(item), nil
}

func (s *SigningKeyService) Status(ctx context.Context, vuzID string) (*model.SigningKeyStatusResponse, error) {
	item, err := s.keys.FindByVUZID(ctx, vuzID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSigningKeyNotFound
		}
		return nil, err
	}

	return toSigningKeyStatusResponse(item), nil
}

func toSigningKeyStatusResponse(item *model.UniversitySigningKey) *model.SigningKeyStatusResponse {
	return &model.SigningKeyStatusResponse{
		Configured:           true,
		KeyAlgorithm:         item.KeyAlgorithm,
		EncryptionAlgorithm:  item.EncryptionAlgorithm,
		PublicKeyFingerprint: item.PublicKeyFingerprint,
		UpdatedAt:            item.UpdatedAt,
	}
}
