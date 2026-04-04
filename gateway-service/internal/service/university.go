package service

import (
	"context"

	"github.com/diasoft/gateway-service/internal/model"
)

type UniversityProfileRepository interface {
	FindByID(ctx context.Context, id string) (*model.University, error)
}

type UniversityBatchRepository interface {
	ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error)
}

type UniversityCabinetService struct {
	universities UniversityProfileRepository
	batches      UniversityBatchRepository
}

func NewUniversityCabinetService(universities UniversityProfileRepository, batches UniversityBatchRepository) *UniversityCabinetService {
	return &UniversityCabinetService{
		universities: universities,
		batches:      batches,
	}
}

func (s *UniversityCabinetService) Profile(ctx context.Context, vuzID string) (*model.University, error) {
	return s.universities.FindByID(ctx, vuzID)
}

func (s *UniversityCabinetService) ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	return s.batches.ListBatches(ctx, vuzID, limit)
}
