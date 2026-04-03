package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
	"github.com/lib/pq"
)

type AuthUniversityRepository interface {
	Create(ctx context.Context, request *model.RegisterUniversityRequest, passwordHash string) (*model.University, error)
	FindByEmail(ctx context.Context, email string) (*model.University, error)
}

type AuthAdminRepository interface {
	FindByEmail(ctx context.Context, email string) (*model.PlatformAdmin, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(plain, hashed string) bool
}

type AccessTokenManager interface {
	IssueAccessToken(subject, vuzID, email, role, status string) (string, time.Time, error)
}

type AuthService struct {
	universities AuthUniversityRepository
	admins       AuthAdminRepository
	hasher       PasswordHasher
	tokens       AccessTokenManager
}

func NewAuthService(universities AuthUniversityRepository, admins AuthAdminRepository, hasher PasswordHasher, tokens AccessTokenManager) *AuthService {
	return &AuthService{
		universities: universities,
		admins:       admins,
		hasher:       hasher,
		tokens:       tokens,
	}
}

func (s *AuthService) Register(ctx context.Context, request *model.RegisterUniversityRequest) (*model.RegisterUniversityResponse, error) {
	passwordHash, err := s.hasher.Hash(request.Password)
	if err != nil {
		return nil, err
	}

	university, err := s.universities.Create(ctx, request, passwordHash)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, ErrDuplicateEntity
		}
		return nil, err
	}

	return &model.RegisterUniversityResponse{
		ID:        university.ID,
		Status:    university.Status,
		CreatedAt: university.CreatedAt,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, request *model.LoginRequest) (*model.AuthResponse, error) {
	admin, err := s.admins.FindByEmail(ctx, request.Email)
	switch {
	case err == nil:
		if !s.hasher.Compare(request.Password, admin.PasswordHash) {
			return nil, ErrInvalidCredentials
		}

		tokenValue, expiresAt, err := s.tokens.IssueAccessToken(admin.ID, "", admin.Email, model.RoleAdmin, model.UniversityStatusActive)
		if err != nil {
			return nil, err
		}

		return &model.AuthResponse{
			AccessToken: tokenValue,
			TokenType:   "Bearer",
			ExpiresAt:   expiresAt,
			Role:        model.RoleAdmin,
			Status:      model.UniversityStatusActive,
			Email:       admin.Email,
		}, nil
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}

	university, err := s.universities.FindByEmail(ctx, request.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !s.hasher.Compare(request.Password, university.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	switch university.Status {
	case model.UniversityStatusPending:
		return nil, ErrUniversityPending
	case model.UniversityStatusBlocked:
		return nil, ErrUniversityBlocked
	case model.UniversityStatusActive:
	default:
		return nil, ErrUniversityInactive
	}

	tokenValue, expiresAt, err := s.tokens.IssueAccessToken(university.ID, university.ID, university.Email, model.RoleUniversity, university.Status)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		AccessToken: tokenValue,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		Role:        model.RoleUniversity,
		Status:      university.Status,
		VUZID:       university.ID,
		Email:       university.Email,
	}, nil
}
