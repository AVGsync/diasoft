package repository

import (
	"context"

	"pubver/internal/domain"
)

type VerificationRepository interface {
	FindByHash(ctx context.Context, hash string) (*domain.DiplomaRecord, error)
	FindByDiplomaNumber(ctx context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error)
	// FindUniversityVerificationKeyByID returns ErrUniversityVerificationKeyNotFound
	// when no verification key exists for the requested VUZ.
	FindUniversityVerificationKeyByID(ctx context.Context, vuzID string) (*domain.UniversityVerificationKey, error)
}
