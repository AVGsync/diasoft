package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"pubver/internal/domain"
	"pubver/internal/testutil"
	"pubver/pkg/verifyhash"
)

type fakeRepository struct {
	verificationKeys map[string]*domain.UniversityVerificationKey
	byHash           map[string]*domain.DiplomaRecord
	byDiploma        map[string]*domain.DiplomaRecord
}

func (r *fakeRepository) FindUniversityVerificationKeyByID(_ context.Context, vuzID string) (*domain.UniversityVerificationKey, error) {
	return r.verificationKeys[vuzID], nil
}

func (r *fakeRepository) FindByHash(_ context.Context, hash string) (*domain.DiplomaRecord, error) {
	return r.byHash[hash], nil
}

func (r *fakeRepository) FindByDiplomaNumber(_ context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error) {
	return r.byDiploma[vuzCode+"::"+diplomaNumber], nil
}

func TestVerifyPayloadReturnsActiveDiploma(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa keypair: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	hash, err := verifyhash.HashDiplomaInput(verifyhash.DiplomaHashInput{
		FullName:      "Ivanov Ivan Ivanovich",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "pepper",
	})
	if err != nil {
		t.Fatalf("hash diploma input: %v", err)
	}

	payload := `{"sub":"` + hash + `","diploma_hash":"` + hash + `","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","year":2024,"salt":"pepper"}`
	token, err := testutil.CreateRS256TestJWT(privateKey, payload)
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	service := NewVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			"550e8400-e29b-41d4-a716-446655440000": {
				PublicKey: string(publicKeyPEM),
			},
		},
		byHash: map[string]*domain.DiplomaRecord{
			hash: {
				Hash:          hash,
				DiplomaNumber: "DVS-2024-001234",
				Status:        domain.DiplomaStatusActive,
				University: domain.University{
					Code: "bmstu",
					Name: "Bauman Moscow State Technical University",
				},
			},
		},
	})

	response, err := service.VerifyPayload(context.Background(), token)
	if err != nil {
		t.Fatalf("verify payload: %v", err)
	}

	if !response.Valid {
		t.Fatalf("expected valid diploma")
	}

	if response.Status != domain.DiplomaStatusActive {
		t.Fatalf("unexpected status: %s", response.Status)
	}
}

func TestSearchReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := NewVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{},
		byDiploma:        map[string]*domain.DiplomaRecord{},
	})

	response, err := service.Search(context.Background(), "bmstu", "AB-42")
	if err != nil {
		t.Fatalf("search diploma: %v", err)
	}

	if response.Valid {
		t.Fatalf("expected not found diploma to be invalid")
	}

	if response.Status != domain.DiplomaStatusNotFound {
		t.Fatalf("unexpected status: %s", response.Status)
	}
}
