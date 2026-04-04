package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/diasoft/gateway-service/internal/infrastructure/security"
	"github.com/diasoft/gateway-service/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

type VerificationRepository interface {
	GetVerificationSnapshot(ctx context.Context, diplomaHash string) (*model.VerificationSnapshot, error)
}

type VerifyService struct {
	repo      VerificationRepository
	qrPayload QRPayloadDecoder
}

type QRPayloadDecoder interface {
	Parse(payload string) (*security.ParsedQRPayload, error)
	ParseUnverifiedEnvelope(tokenString string) (*security.QRPayloadEnvelope, error)
}

func NewVerifyService(repo VerificationRepository, qrPayload QRPayloadDecoder) *VerifyService {
	return &VerifyService{repo: repo, qrPayload: qrPayload}
}

func (s *VerifyService) VerifyQRCode(ctx context.Context, payload string) (*model.VerifyResponse, error) {
	envelope, err := s.qrPayload.ParseUnverifiedEnvelope(payload)
	if err != nil {
		return nil, err
	}

	diplomaHash := firstNonEmpty(envelope.DiplomaHash, envelope.Subject)

	snapshot, err := s.repo.GetVerificationSnapshot(ctx, diplomaHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &model.VerifyResponse{
				Valid:       false,
				DiplomaHash: diplomaHash,
				Message:     "diploma not found",
			}, nil
		}
		return nil, err
	}

	response := &model.VerifyResponse{
		Valid:         false,
		Status:        snapshot.Status,
		DiplomaHash:   snapshot.DiplomaHash,
		DiplomaNumber: snapshot.DiplomaNumber,
		StudentName:   snapshot.FullName,
		Specialty:     snapshot.Specialty,
		Degree:        snapshot.Degree,
		Faculty:       snapshot.Faculty,
		Year:          snapshot.Year,
		UniversityID:  snapshot.UniversityID,
		University:    snapshot.UniversityName,
		CreatedAt:     snapshot.CreatedAt,
	}

	if snapshot.PublicKey != nil {
		publicKey, publicKeyErr := security.ParseEd25519PublicKeyPEM(*snapshot.PublicKey)
		if publicKeyErr == nil {
			_, err = jwt.Parse(payload, func(token *jwt.Token) (interface{}, error) {
				if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
					return nil, errors.New("qr token must use EdDSA")
				}
				return publicKey, nil
			}, jwt.WithoutClaimsValidation())
			response.JWTSignatureValid = err == nil
		}
	}

	decryptedPayload, err := s.qrPayload.Parse(payload)
	if err != nil {
		response.Message = "diploma verification failed"
		return response, nil
	}

	raw := fmt.Sprintf(
		"%s|%s|%s|%d|%s|%s",
		decryptedPayload.FullName,
		decryptedPayload.DiplomaNumber,
		decryptedPayload.Specialty,
		decryptedPayload.Year,
		decryptedPayload.VUZID,
		decryptedPayload.Salt,
	)
	sum := sha256.Sum256([]byte(raw))
	expectedHash := hex.EncodeToString(sum[:])
	response.HashMatches = expectedHash == snapshot.DiplomaHash && expectedHash == diplomaHash

	response.Valid = response.HashMatches && snapshot.Status == model.DiplomaStatusActive && response.JWTSignatureValid
	if response.Valid {
		response.Message = "diploma is valid"
	} else {
		response.Message = "diploma verification failed"
	}

	return response, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
