package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/diasoft/gateway-service/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

type VerificationRepository interface {
	GetVerificationSnapshot(ctx context.Context, diplomaHash string) (*model.VerificationSnapshot, error)
}

type VerifyService struct {
	repo VerificationRepository
}

func NewVerifyService(repo VerificationRepository) *VerifyService {
	return &VerifyService{repo: repo}
}

func (s *VerifyService) VerifyQRCode(ctx context.Context, payload string) (*model.VerifyResponse, error) {
	unverifiedClaims := jwt.MapClaims{}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	_, _, err := parser.ParseUnverified(payload, unverifiedClaims)
	if err != nil {
		return nil, err
	}

	diplomaHash := stringValue(unverifiedClaims["diploma_hash"])
	if diplomaHash == "" {
		diplomaHash = stringValue(unverifiedClaims["sub"])
	}

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

	parsedPublicKey, publicKeyErr := parsePublicKey(snapshot.PublicKey)
	if publicKeyErr == nil && parsedPublicKey != nil {
		_, err = jwt.Parse(payload, func(token *jwt.Token) (interface{}, error) {
			switch token.Method.Alg() {
			case jwt.SigningMethodRS256.Alg():
				publicKey, ok := parsedPublicKey.(*rsa.PublicKey)
				if !ok {
					return nil, errors.New("qr token expects RSA public key")
				}
				return publicKey, nil
			case jwt.SigningMethodEdDSA.Alg():
				publicKey, ok := parsedPublicKey.(ed25519.PublicKey)
				if !ok {
					return nil, errors.New("qr token expects Ed25519 public key")
				}
				return publicKey, nil
			default:
				return nil, fmt.Errorf("unsupported qr token algorithm %s", token.Method.Alg())
			}
		}, jwt.WithoutClaimsValidation())
		response.JWTSignatureValid = err == nil
	}

	studentName := stringValue(unverifiedClaims["student_name"])
	if studentName == "" {
		studentName = stringValue(unverifiedClaims["full_name"])
	}
	diplomaNumber := stringValue(unverifiedClaims["diploma_number"])
	specialty := stringValue(unverifiedClaims["specialty"])
	vuzID := stringValue(unverifiedClaims["vuz_id"])
	salt := stringValue(unverifiedClaims["salt"])

	year, err := intValue(unverifiedClaims["year"])
	if err != nil {
		return response, nil
	}

	raw := fmt.Sprintf("%s|%s|%s|%d|%s|%s", studentName, diplomaNumber, specialty, year, vuzID, salt)
	sum := sha256.Sum256([]byte(raw))
	expectedHash := hex.EncodeToString(sum[:])
	response.HashMatches = expectedHash == snapshot.DiplomaHash && expectedHash == diplomaHash

	if parsedPublicKey != nil && snapshot.Signature != nil {
		signatureBytes, sigErr := base64.StdEncoding.DecodeString(*snapshot.Signature)
		if sigErr == nil {
			if publicKey, ok := parsedPublicKey.(ed25519.PublicKey); ok {
				response.HashSignatureValid = ed25519.Verify(publicKey, []byte(snapshot.DiplomaHash), signatureBytes)
			}
		}
	}

	response.Valid = response.HashMatches && snapshot.Status == model.DiplomaStatusActive && (response.JWTSignatureValid || response.HashSignatureValid)
	if response.Valid {
		response.Message = "diploma is valid"
	} else {
		response.Message = "diploma verification failed"
	}

	return response, nil
}

func parsePublicKey(value *string) (interface{}, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, errors.New("public key is empty")
	}

	block, _ := pem.Decode([]byte(*value))
	if block == nil {
		return nil, errors.New("failed to decode public key pem")
	}

	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, errors.New("unsupported public key format")
}

func stringValue(value interface{}) string {
	if cast, ok := value.(string); ok {
		return cast
	}
	return ""
}

func intValue(value interface{}) (int, error) {
	switch cast := value.(type) {
	case float64:
		return int(cast), nil
	case int:
		return cast, nil
	case string:
		return strconv.Atoi(cast)
	default:
		return 0, errors.New("unsupported numeric value")
	}
}
