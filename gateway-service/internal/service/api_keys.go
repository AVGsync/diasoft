package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/diasoft/gateway-service/internal/model"
)

type APIKeyRepository interface {
	Create(ctx context.Context, vuzID string, name *string, keyHash string) (*model.APIKey, error)
	ListActiveByVUZ(ctx context.Context, vuzID string) ([]*model.APIKeySummary, error)
	FindActiveUniversityByHash(ctx context.Context, keyHash string) (*model.University, *model.APIKey, error)
}

type APIKeyService struct {
	repo APIKeyRepository
}

func NewAPIKeyService(repo APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

func (s *APIKeyService) Create(ctx context.Context, vuzID string, name *string) (*model.CreateAPIKeyResponse, error) {
	rawKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}

	keyHash := hashAPIKey(rawKey)
	item, err := s.repo.Create(ctx, vuzID, name, keyHash)
	if err != nil {
		return nil, err
	}

	return &model.CreateAPIKeyResponse{
		ID:        item.ID,
		Name:      item.Name,
		APIKey:    rawKey,
		CreatedAt: item.CreatedAt,
	}, nil
}

func (s *APIKeyService) List(ctx context.Context, vuzID string) ([]*model.APIKeySummary, error) {
	return s.repo.ListActiveByVUZ(ctx, vuzID)
}

func (s *APIKeyService) ResolveActiveUniversity(ctx context.Context, plainKey string) (*model.University, *model.APIKey, error) {
	return s.repo.FindActiveUniversityByHash(ctx, hashAPIKey(plainKey))
}

func generateAPIKey() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return "vuz_" + hex.EncodeToString(buffer), nil
}

func hashAPIKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
