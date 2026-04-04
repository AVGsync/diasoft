package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"pubver/internal/domain"
	"pubver/internal/repository"
	"pubver/pkg/verifyhash"
)

type VerificationService struct {
	repo   repository.VerificationRepository
	logger *slog.Logger
}

func NewVerificationService(repo repository.VerificationRepository, logger *slog.Logger) *VerificationService {
	if logger == nil {
		logger = slog.Default()
	}

	return &VerificationService{
		repo:   repo,
		logger: logger,
	}
}

func (s *VerificationService) VerifyPayload(ctx context.Context, token string) (domain.VerifyResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.VerifyResponse{}, domain.ErrInvalidInput
	}

	claimsMap, err := verifyhash.DecodeUnverifiedJWT(token)
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "decode_unverified_jwt", "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	vuzID, err := verifyhash.ExtractVUZIDFromMap(claimsMap)
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "missing_vuz_id", "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	verificationKey, err := s.repo.FindUniversityVerificationKeyByID(ctx, vuzID)
	if err != nil {
		if errors.Is(err, domain.ErrUniversityVerificationKeyNotFound) {
			s.logger.Warn("verification payload rejected", "reason", "verification_key_not_found", "vuz_id", vuzID)
			return domain.VerifyResponse{}, domain.ErrInvalidPayload
		}

		s.logger.Error("load university verification key", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, err
	}

	if err := verifyhash.VerifyEd25519JWT(token, verificationKey.PublicKey); err != nil {
		s.logger.Warn("verification payload rejected", "reason", "invalid_signature", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	claims, err := verifyhash.ExtractQRClaimsFromMap(claimsMap)
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "invalid_claims", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	// The registry lookup always uses the recomputed hash, even when optional
	// hash claims are absent from the JWT for backward compatibility.
	hash, err := verifyhash.HashDiplomaInput(claims.HashInput())
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "hash_recompute_failed", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}
	if claims.DiplomaHash != "" && !strings.EqualFold(claims.DiplomaHash, hash) {
		s.logger.Warn("verification payload rejected", "reason", "diploma_hash_mismatch", "vuz_id", vuzID)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, domain.ErrDiplomaHashClaimMismatch)
	}
	if claims.Sub != "" && !strings.EqualFold(claims.Sub, hash) {
		s.logger.Warn("verification payload rejected", "reason", "sub_mismatch", "vuz_id", vuzID)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, domain.ErrSubClaimMismatch)
	}

	record, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		s.logger.Error("find diploma by hash", "hash", hash, "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, err
	}

	if record == nil {
		s.logger.Warn("diploma not found in registry", "hash", hash, "vuz_id", vuzID)
		return domain.VerifyResponse{
			Valid:  false,
			Status: domain.DiplomaStatusNotFound,
			Hash:   hash,
		}, nil
	}

	return toVerifyResponse(record), nil
}

func (s *VerificationService) Search(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error) {
	vuzCode = strings.TrimSpace(vuzCode)
	diplomaNumber = strings.TrimSpace(diplomaNumber)
	if vuzCode == "" || diplomaNumber == "" {
		return domain.SearchResponse{}, domain.ErrInvalidInput
	}

	record, err := s.repo.FindByDiplomaNumber(ctx, vuzCode, diplomaNumber)
	if err != nil {
		s.logger.Error("find diploma by number", "vuz_code", vuzCode, "diploma_number", diplomaNumber, "error", err)
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
