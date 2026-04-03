package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/infrastructure/token"
	"github.com/diasoft/gateway-service/internal/model"
)

type StudentRepository interface {
	SearchStudents(ctx context.Context, diplomaNumber, fullName string) ([]*model.StudentSearchResult, error)
	FindStudentByHash(ctx context.Context, diplomaHash string) (*model.StudentSearchResult, error)
	CreateShareLink(ctx context.Context, diplomaHash, token string, expiresAt time.Time) error
	FindShareLink(ctx context.Context, token string) (*model.ShareLink, error)
	IncrementShareLinkUsage(ctx context.Context, token string) error
}

type ShareTokenManager interface {
	IssueShareToken(diplomaHash string, ttl time.Duration) (string, time.Time, error)
	ParseShareToken(tokenString string) (*token.ShareClaims, error)
}

type QRGenerator interface {
	PNG(content string, size int) ([]byte, error)
	SVG(content string, moduleSize int) ([]byte, error)
}

type StudentService struct {
	repo          StudentRepository
	tokens        ShareTokenManager
	qr            QRGenerator
	publicBaseURL string
	defaultTTL    time.Duration
}

func NewStudentService(repo StudentRepository, tokens ShareTokenManager, qr QRGenerator, publicBaseURL string, defaultTTL time.Duration) *StudentService {
	return &StudentService{
		repo:          repo,
		tokens:        tokens,
		qr:            qr,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		defaultTTL:    defaultTTL,
	}
}

func (s *StudentService) Search(ctx context.Context, diplomaNumber, fullName string) ([]*model.StudentSearchResult, error) {
	return s.repo.SearchStudents(ctx, diplomaNumber, fullName)
}

func (s *StudentService) CreateShareLink(ctx context.Context, diplomaHash string, ttl time.Duration) (*model.ShareLinkResponse, error) {
	if _, err := s.repo.FindStudentByHash(ctx, diplomaHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDiplomaNotFound
		}
		return nil, err
	}

	if ttl <= 0 {
		ttl = s.defaultTTL
	}

	tokenValue, expiresAt, err := s.tokens.IssueShareToken(diplomaHash, ttl)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreateShareLink(ctx, diplomaHash, tokenValue, expiresAt); err != nil {
		return nil, err
	}

	return &model.ShareLinkResponse{
		ShareURL:  fmt.Sprintf("%s/api/v1/student/share/%s", s.publicBaseURL, tokenValue),
		Token:     tokenValue,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *StudentService) RenderShareQR(ctx context.Context, diplomaHash, format string, ttl time.Duration) ([]byte, string, error) {
	link, err := s.CreateShareLink(ctx, diplomaHash, ttl)
	if err != nil {
		return nil, "", err
	}

	switch strings.ToLower(format) {
	case "", "png":
		payload, err := s.qr.PNG(link.ShareURL, 220)
		return payload, "image/png", err
	case "svg":
		payload, err := s.qr.SVG(link.ShareURL, 8)
		return payload, "image/svg+xml", err
	default:
		payload, err := s.qr.PNG(link.ShareURL, 220)
		return payload, "image/png", err
	}
}

func (s *StudentService) ResolveShareLink(ctx context.Context, tokenValue string) (*model.SharedDiplomaResponse, error) {
	claims, err := s.tokens.ParseShareToken(tokenValue)
	if err != nil {
		return nil, ErrInvalidShareToken
	}

	link, err := s.repo.FindShareLink(ctx, tokenValue)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidShareToken
		}
		return nil, err
	}

	if link.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrShareLinkExpired
	}
	if link.DiplomaHash != claims.DiplomaHash {
		return nil, ErrInvalidShareToken
	}

	student, err := s.repo.FindStudentByHash(ctx, claims.DiplomaHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDiplomaNotFound
		}
		return nil, err
	}

	if err := s.repo.IncrementShareLinkUsage(ctx, tokenValue); err != nil {
		return nil, err
	}

	return &model.SharedDiplomaResponse{
		DiplomaHash:    student.DiplomaHash,
		DiplomaNumber:  student.DiplomaNumber,
		FullName:       student.FullName,
		Specialty:      student.Specialty,
		Degree:         student.Degree,
		Faculty:        student.Faculty,
		Year:           student.Year,
		UniversityID:   student.UniversityID,
		UniversityName: student.UniversityName,
		Status:         student.Status,
		ExpiresAt:      link.ExpiresAt,
	}, nil
}
