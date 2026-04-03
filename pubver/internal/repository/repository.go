package repository

import (
	"context"

	"pubver/internal/domain"
)

type VerificationRepository interface {
	FindByHash(ctx context.Context, hash string) (*domain.DiplomaRecord, error)
	FindByDiplomaNumber(ctx context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error)
	FindUniversityVerificationKeyByID(ctx context.Context, vuzID string) (*domain.UniversityVerificationKey, error)
}
