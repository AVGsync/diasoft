package stub

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"pubver/internal/domain"
	"pubver/pkg/verifyhash"
)

type Scenario struct {
	ActiveSearchVUZCode        string
	ActiveSearchDiplomaNumber  string
	ActiveVerifyToken          string
	RevokedSearchVUZCode       string
	RevokedSearchDiplomaNumber string
	RevokedVerifyToken         string
}

type VerificationRepository struct {
	verificationKeys map[string]*domain.UniversityVerificationKey
	byHash           map[string]*domain.DiplomaRecord
	byDiploma        map[string]*domain.DiplomaRecord
}

func NewVerificationRepository() (*VerificationRepository, Scenario, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, Scenario{}, fmt.Errorf("generate stub rsa keypair: %w", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, Scenario{}, fmt.Errorf("marshal stub public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	repo := &VerificationRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{},
		byHash:           map[string]*domain.DiplomaRecord{},
		byDiploma:        map[string]*domain.DiplomaRecord{},
	}

	activeScenario, err := repo.addScenario(privateKey, string(publicKeyPEM), scenarioInput{
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		VUZCode:       "001X7276",
		University:    "Bauman Moscow State Technical University",
		FullName:      "Иванов Иван Иванович",
		DiplomaNumber: "ДВС-2024-001234",
		Specialty:     "Программная инженерия",
		Year:          2024,
		Salt:          "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Status:        domain.DiplomaStatusActive,
	})
	if err != nil {
		return nil, Scenario{}, err
	}

	revokedScenario, err := repo.addScenario(privateKey, string(publicKeyPEM), scenarioInput{
		VUZID:         "550e8400-e29b-41d4-a716-446655440001",
		VUZCode:       "002X7277",
		University:    "Saint Petersburg State University",
		FullName:      "Петров Петр Петрович",
		DiplomaNumber: "ДВС-2023-009999",
		Specialty:     "Информационные системы",
		Year:          2023,
		Salt:          "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Status:        domain.DiplomaStatusRevoked,
	})
	if err != nil {
		return nil, Scenario{}, err
	}

	return repo, Scenario{
		ActiveSearchVUZCode:        activeScenario.VUZCode,
		ActiveSearchDiplomaNumber:  activeScenario.DiplomaNumber,
		ActiveVerifyToken:          activeScenario.Token,
		RevokedSearchVUZCode:       revokedScenario.VUZCode,
		RevokedSearchDiplomaNumber: revokedScenario.DiplomaNumber,
		RevokedVerifyToken:         revokedScenario.Token,
	}, nil
}

func (r *VerificationRepository) FindByHash(_ context.Context, hash string) (*domain.DiplomaRecord, error) {
	return r.byHash[hash], nil
}

func (r *VerificationRepository) FindByDiplomaNumber(_ context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error) {
	return r.byDiploma[vuzCode+"::"+diplomaNumber], nil
}

func (r *VerificationRepository) FindUniversityVerificationKeyByID(_ context.Context, vuzID string) (*domain.UniversityVerificationKey, error) {
	return r.verificationKeys[vuzID], nil
}

type scenarioInput struct {
	VUZID         string
	VUZCode       string
	University    string
	FullName      string
	DiplomaNumber string
	Specialty     string
	Year          int
	Salt          string
	Status        domain.DiplomaStatus
}

type scenarioResult struct {
	VUZCode       string
	DiplomaNumber string
	Token         string
}

func (r *VerificationRepository) addScenario(privateKey *rsa.PrivateKey, publicKeyPEM string, input scenarioInput) (scenarioResult, error) {
	hash, err := verifyhash.HashDiplomaInput(verifyhash.DiplomaHashInput{
		FullName:      input.FullName,
		DiplomaNumber: input.DiplomaNumber,
		Specialty:     input.Specialty,
		Year:          input.Year,
		VUZID:         input.VUZID,
		Salt:          input.Salt,
	})
	if err != nil {
		return scenarioResult{}, fmt.Errorf("hash stub diploma input: %w", err)
	}

	token, err := createRS256JWT(privateKey, map[string]any{
		"sub":            hash,
		"diploma_hash":   hash,
		"vuz_id":         input.VUZID,
		"diploma_number": input.DiplomaNumber,
		"student_name":   input.FullName,
		"specialty":      input.Specialty,
		"year":           input.Year,
		"salt":           input.Salt,
		"iat":            time.Now().Unix(),
	})
	if err != nil {
		return scenarioResult{}, fmt.Errorf("create stub jwt: %w", err)
	}

	record := &domain.DiplomaRecord{
		Hash:          hash,
		DiplomaNumber: input.DiplomaNumber,
		Status:        input.Status,
		University: domain.University{
			Code: input.VUZCode,
			Name: input.University,
		},
	}

	if input.Status == domain.DiplomaStatusRevoked {
		revokedAt := time.Date(2025, time.January, 15, 10, 0, 0, 0, time.UTC)
		record.RevokedAt = &revokedAt
	}

	r.verificationKeys[input.VUZID] = &domain.UniversityVerificationKey{PublicKey: publicKeyPEM}
	r.byHash[hash] = record
	r.byDiploma[input.VUZCode+"::"+input.DiplomaNumber] = record

	return scenarioResult{
		VUZCode:       input.VUZCode,
		DiplomaNumber: input.DiplomaNumber,
		Token:         token,
	}, nil
}

func createRS256JWT(privateKey *rsa.PrivateKey, payload map[string]any) (string, error) {
	headerEncoded := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	bodyEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := headerEncoded + "." + bodyEncoded

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
