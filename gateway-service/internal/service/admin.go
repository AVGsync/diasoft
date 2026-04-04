package service

import (
	"context"

	"github.com/diasoft/gateway-service/internal/model"
)

type AdminRepository interface {
	UpsertBootstrap(ctx context.Context, email, passwordHash string) error
	Stats(ctx context.Context) (*model.AdminStats, error)
}

type UniversityAdminRepository interface {
	Activate(ctx context.Context, id string) (*model.University, error)
	FindByID(ctx context.Context, id string) (*model.University, error)
	List(ctx context.Context) ([]*model.University, error)
	UpdateStatus(ctx context.Context, id, status string) (*model.University, error)
	DeleteCascade(ctx context.Context, id string) error
}

type AdminService struct {
	admins       AdminRepository
	universities UniversityAdminRepository
	hasher       PasswordHasher
}

func NewAdminService(admins AdminRepository, universities UniversityAdminRepository, hasher PasswordHasher) *AdminService {
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

func (s *AdminService) GetUniversity(ctx context.Context, id string) (*model.University, error) {
	return s.universities.FindByID(ctx, id)
}

func (s *AdminService) ListUniversities(ctx context.Context) ([]*model.University, error) {
	return s.universities.List(ctx)
}

func (s *AdminService) UpdateUniversityStatus(ctx context.Context, id, status string) (*model.University, error) {
	return s.universities.UpdateStatus(ctx, id, status)
}

func (s *AdminService) DeleteUniversity(ctx context.Context, id string) error {
	return s.universities.DeleteCascade(ctx, id)
}

func (s *AdminService) Stats(ctx context.Context) (*model.AdminStats, error) {
	return s.admins.Stats(ctx)
}
