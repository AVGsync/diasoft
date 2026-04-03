package service

import (
	"context"
	"errors"
	"strings"

	"pubver/internal/domain"
	"pubver/internal/repository"
	"pubver/pkg/verifyhash"
)

type VerificationService struct {
	repo repository.VerificationRepository
}

func NewVerificationService(repo repository.VerificationRepository) *VerificationService {
	return &VerificationService{
		repo: repo,
	}
}

func (s *VerificationService) VerifyPayload(ctx context.Context, token string) (domain.VerifyResponse, error) {
	if strings.TrimSpace(token) == "" {
		return domain.VerifyResponse{}, domain.ErrInvalidInput
	}

	vuzID, err := verifyhash.ExtractVUZID(token)
	if err != nil {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	verificationKey, err := s.repo.FindUniversityVerificationKeyByID(ctx, vuzID)
	if err != nil {
		return domain.VerifyResponse{}, err
	}
	if verificationKey == nil {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, errors.New("university for vuz_id not found"))
	}

	if err := verifyhash.VerifyRS256JWT(token, verificationKey.PublicKey); err != nil {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	claims, err := verifyhash.ExtractQRClaims(token)
	if err != nil {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	hash, err := verifyhash.HashDiplomaInput(claims.HashInput())
	if err != nil {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}
	if claims.DiplomaHash != "" && !strings.EqualFold(claims.DiplomaHash, hash) {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, errors.New("diploma_hash claim does not match recomputed hash"))
	}
	if claims.Sub != "" && !strings.EqualFold(claims.Sub, hash) {
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, errors.New("sub claim does not match recomputed hash"))
	}

	record, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		return domain.VerifyResponse{}, err
	}

	if record == nil {
		return domain.VerifyResponse{
			Valid:  false,
			Status: domain.DiplomaStatusNotFound,
			Hash:   hash,
		}, nil
	}

	return toVerifyResponse(record), nil
}

func (s *VerificationService) Search(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error) {
	if strings.TrimSpace(vuzCode) == "" || strings.TrimSpace(diplomaNumber) == "" {
		return domain.SearchResponse{}, domain.ErrInvalidInput
	}

	record, err := s.repo.FindByDiplomaNumber(ctx, strings.TrimSpace(vuzCode), strings.TrimSpace(diplomaNumber))
	if err != nil {
		return domain.SearchResponse{}, err
	}

	if record == nil {
		return domain.SearchResponse{
			Valid:  false,
			Status: domain.DiplomaStatusNotFound,
		}, nil
	}

	return domain.SearchResponse{
		Valid:      record.Status == domain.DiplomaStatusActive,
		Status:     record.Status,
		University: record.University.Name,
		VUZCode:    record.University.Code,
		Year:       record.GraduateYear,
		Specialty:  record.Specialty,
	}, nil
}

func toVerifyResponse(record *domain.DiplomaRecord) domain.VerifyResponse {
	return domain.VerifyResponse{
		Valid:         record.Status == domain.DiplomaStatusActive,
		Status:        record.Status,
		Hash:          record.Hash,
		DiplomaNumber: record.DiplomaNumber,
		University:    record.University.Name,
		VUZCode:       record.University.Code,
		Year:          record.GraduateYear,
		Specialty:     record.Specialty,
		RevokedAt:     record.RevokedAt,
	}
}
