package service

import (
	"context"

	"github.com/diasoft/gateway-service/internal/model"
)

type AdminRepository interface {
	UpsertBootstrap(ctx context.Context, email, passwordHash string) error
	Stats(ctx context.Context) (*model.AdminStats, error)
}

type UniversityActivator interface {
	Activate(ctx context.Context, id string) (*model.University, error)
}

type AdminService struct {
	admins       AdminRepository
	universities UniversityActivator
	hasher       PasswordHasher
}

func NewAdminService(admins AdminRepository, universities UniversityActivator, hasher PasswordHasher) *AdminService {
	return &AdminService{
		admins:       admins,
		universities: universities,
		hasher:       hasher,
	}
}

func (s *AdminService) EnsureBootstrapAdmin(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}

	return s.admins.UpsertBootstrap(ctx, email, passwordHash)
}

func (s *AdminService) ApproveUniversity(ctx context.Context, id string) (*model.University, error) {
	return s.universities.Activate(ctx, id)
}

func (s *AdminService) Stats(ctx context.Context) (*model.AdminStats, error) {
	return s.admins.Stats(ctx)
}
