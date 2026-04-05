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
	encKey []byte
}

func NewVerificationService(repo repository.VerificationRepository, logger *slog.Logger, encKey []byte) *VerificationService {
	if logger == nil {
		logger = slog.Default()
	}

	return &VerificationService{
		repo:   repo,
		logger: logger,
		encKey: append([]byte(nil), encKey...),
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

	outerClaims, err := verifyhash.ExtractOuterQRClaimsFromMap(claimsMap)
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "invalid_outer_claims", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	diplomaPayload, err := verifyhash.DecryptEncryptedDiplomaPayload(outerClaims.Enc, s.encKey)
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "decrypt_enc_failed", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}

	hash, err := verifyhash.HashDiplomaInput(diplomaPayload.HashInput(outerClaims.VUZID))
	if err != nil {
		s.logger.Warn("verification payload rejected", "reason", "hash_recompute_failed", "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, err)
	}
	if !strings.EqualFold(outerClaims.DiplomaHash, hash) {
		s.logger.Warn("verification payload rejected", "reason", "diploma_hash_mismatch", "vuz_id", vuzID)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, domain.ErrDiplomaHashClaimMismatch)
	}
	if !strings.EqualFold(outerClaims.Sub, hash) {
		s.logger.Warn("verification payload rejected", "reason", "sub_mismatch", "vuz_id", vuzID)
		return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, domain.ErrSubClaimMismatch)
	}

	record, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrDiplomaRecordNotFound) {
			s.logger.Warn("diploma not found in registry", "hash", hash, "vuz_id", vuzID)
			return domain.VerifyResponse{
				Valid:  false,
				Status: domain.DiplomaStatusNotFound,
				Hash:   hash,
			}, nil
		}

		s.logger.Error("find diploma by hash", "hash", hash, "vuz_id", vuzID, "error", err)
		return domain.VerifyResponse{}, err
	}

	applyEncryptedPayload(record, diplomaPayload)
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
		if errors.Is(err, domain.ErrDiplomaRecordNotFound) {
			return domain.SearchResponse{
				Valid:  false,
				Status: domain.DiplomaStatusNotFound,
			}, nil
		}

		s.logger.Error("find diploma by number", "vuz_code", vuzCode, "diploma_number", diplomaNumber, "error", err)
		return domain.SearchResponse{}, err
	}

	if err := s.enrichRecordFromStoredQR(record); err != nil {
		s.logger.Warn("enrich diploma metadata from stored qr", "vuz_code", vuzCode, "diploma_number", diplomaNumber, "error", err)
	}

	return domain.SearchResponse{
		Valid:      record.Status == domain.DiplomaStatusActive,
		Status:     record.Status,
		University: record.University.Name,
		VUZCode:    record.University.Code,
		Year:       record.GraduateYear,
		Specialty:  record.Specialty,
		Degree:     record.Degree,
		Faculty:    record.Faculty,
	}, nil
}

func (s *VerificationService) enrichRecordFromStoredQR(record *domain.DiplomaRecord) error {
	if record == nil || record.QRPayload == nil || strings.TrimSpace(*record.QRPayload) == "" {
		return nil
	}
	if record.GraduateYear != nil && record.Specialty != nil && record.Degree != nil && record.Faculty != nil {
		return nil
	}

	outerClaims, err := verifyhash.ExtractOuterQRClaims(*record.QRPayload)
	if err != nil {
		return err
	}

	payload, err := verifyhash.DecryptEncryptedDiplomaPayload(outerClaims.Enc, s.encKey)
	if err != nil {
		return err
	}

	applyEncryptedPayload(record, payload)
	return nil
}

func applyEncryptedPayload(record *domain.DiplomaRecord, payload verifyhash.EncryptedDiplomaPayload) {
	if record == nil {
		return
	}

	if strings.TrimSpace(record.DiplomaNumber) == "" {
		record.DiplomaNumber = payload.DiplomaNumber
	}
	if record.GraduateYear == nil && payload.Year != 0 {
		year := payload.Year
		record.GraduateYear = &year
	}
	if record.Specialty == nil && strings.TrimSpace(payload.Specialty) != "" {
		value := payload.Specialty
		record.Specialty = &value
	}
	if record.Degree == nil && strings.TrimSpace(payload.Degree) != "" {
		value := payload.Degree
		record.Degree = &value
	}
	if record.Faculty == nil && strings.TrimSpace(payload.Faculty) != "" {
		value := payload.Faculty
		record.Faculty = &value
	}
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
		Degree:        record.Degree,
		Faculty:       record.Faculty,
		RevokedAt:     record.RevokedAt,
	}
}
