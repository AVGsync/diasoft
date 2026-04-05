package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"pubver/internal/domain"
	"pubver/internal/testutil"
	"pubver/pkg/verifyhash"
)

type fakeRepository struct {
	verificationKey *domain.UniversityVerificationKey
	verificationErr error
	record          *domain.DiplomaRecord
	recordErr       error
	searchRecord    *domain.DiplomaRecord
	searchErr       error
}

func (r fakeRepository) FindByHash(context.Context, string) (*domain.DiplomaRecord, error) {
	return r.record, r.recordErr
}

func (r fakeRepository) FindByDiplomaNumber(context.Context, string, string) (*domain.DiplomaRecord, error) {
	return r.searchRecord, r.searchErr
}

func (r fakeRepository) FindUniversityVerificationKeyByID(context.Context, string) (*domain.UniversityVerificationKey, error) {
	return r.verificationKey, r.verificationErr
}

func TestVerifyPayloadEmptyToken(t *testing.T) {
	service := NewVerificationService(fakeRepository{}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	_, err := service.VerifyPayload(context.Background(), "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("VerifyPayload() error = %v, want ErrInvalidInput", err)
	}
}

func TestVerifyPayloadInvalidJWT(t *testing.T) {
	service := NewVerificationService(fakeRepository{}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	_, err := service.VerifyPayload(context.Background(), "abc")
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("VerifyPayload() error = %v, want ErrInvalidPayload", err)
	}
}

func TestVerifyPayloadMissingVerificationKey(t *testing.T) {
	token := buildValidToken(t, []byte("0123456789abcdef0123456789abcdef"))
	service := NewVerificationService(fakeRepository{
		verificationErr: domain.ErrUniversityVerificationKeyNotFound,
	}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	_, err := service.VerifyPayload(context.Background(), token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("VerifyPayload() error = %v, want ErrInvalidPayload", err)
	}
}

func TestVerifyPayloadInvalidSignature(t *testing.T) {
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	otherKeyPair := testutil.MustGenerateEd25519KeyPair(t)
	encKey := []byte("0123456789abcdef0123456789abcdef")
	token := buildTokenForKey(t, keyPair, encKey, false)

	service := NewVerificationService(fakeRepository{
		verificationKey: &domain.UniversityVerificationKey{PublicKey: otherKeyPair.PublicKeyHex},
	}, slog.Default(), encKey)

	_, err := service.VerifyPayload(context.Background(), token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("VerifyPayload() error = %v, want ErrInvalidPayload", err)
	}
}

func TestVerifyPayloadHashMismatch(t *testing.T) {
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	encKey := []byte("0123456789abcdef0123456789abcdef")
	token := buildTokenForKey(t, keyPair, encKey, true)

	service := NewVerificationService(fakeRepository{
		verificationKey: &domain.UniversityVerificationKey{PublicKey: keyPair.PublicKeyHex},
	}, slog.Default(), encKey)

	_, err := service.VerifyPayload(context.Background(), token)
	if !errors.Is(err, domain.ErrInvalidPayload) || !errors.Is(err, domain.ErrDiplomaHashClaimMismatch) {
		t.Fatalf("VerifyPayload() error = %v, want ErrInvalidPayload + ErrDiplomaHashClaimMismatch", err)
	}
}

func TestVerifyPayloadNotFoundReturnsMinimalResponse(t *testing.T) {
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	encKey := []byte("0123456789abcdef0123456789abcdef")
	token, hash := buildTokenAndHash(t, keyPair, encKey, false)

	service := NewVerificationService(fakeRepository{
		verificationKey: &domain.UniversityVerificationKey{PublicKey: keyPair.PublicKeyHex},
		recordErr:       domain.ErrDiplomaRecordNotFound,
	}, slog.Default(), encKey)

	response, err := service.VerifyPayload(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyPayload() error = %v", err)
	}

	if response.Status != domain.DiplomaStatusNotFound || response.Hash != hash || response.DiplomaNumber != "" || response.Year != nil {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestVerifyPayloadRevoked(t *testing.T) {
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	encKey := []byte("0123456789abcdef0123456789abcdef")
	token, hash := buildTokenAndHash(t, keyPair, encKey, false)
	year := 2024
	specialty := "Software Engineering"
	degree := "Bachelor"
	faculty := "FKN"
	now := time.Now().UTC()

	service := NewVerificationService(fakeRepository{
		verificationKey: &domain.UniversityVerificationKey{PublicKey: keyPair.PublicKeyHex},
		record: &domain.DiplomaRecord{
			Hash:          hash,
			DiplomaNumber: "DVS-2024-001234",
			Status:        domain.DiplomaStatusRevoked,
			University:    domain.University{Name: "Bauman", Code: "001X7276"},
			GraduateYear:  &year,
			Specialty:     &specialty,
			Degree:        &degree,
			Faculty:       &faculty,
			RevokedAt:     &now,
		},
	}, slog.Default(), encKey)

	response, err := service.VerifyPayload(context.Background(), token)
	if err != nil {
		t.Fatalf("VerifyPayload() error = %v", err)
	}

	if response.Valid || response.Status != domain.DiplomaStatusRevoked || response.University != "Bauman" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSearchHappyPath(t *testing.T) {
	year := 2024
	specialty := "Software Engineering"
	degree := "Bachelor"
	faculty := "FKN"

	service := NewVerificationService(fakeRepository{
		searchRecord: &domain.DiplomaRecord{
			Status:       domain.DiplomaStatusActive,
			University:   domain.University{Name: "Bauman", Code: "001X7276"},
			GraduateYear: &year,
			Specialty:    &specialty,
			Degree:       &degree,
			Faculty:      &faculty,
		},
	}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	response, err := service.Search(context.Background(), "001X7276", "DVS-2024-001234")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !response.Valid || response.University != "Bauman" || response.Year == nil || *response.Year != 2024 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSearchNotFound(t *testing.T) {
	service := NewVerificationService(fakeRepository{
		searchErr: domain.ErrDiplomaRecordNotFound,
	}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	response, err := service.Search(context.Background(), "001X7276", "missing")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if response.Status != domain.DiplomaStatusNotFound || response.Valid {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSearchReturnsPartialResponseWhenStoredQRIsInvalid(t *testing.T) {
	service := NewVerificationService(fakeRepository{
		searchRecord: &domain.DiplomaRecord{
			DiplomaNumber: "DIP-2022-0003",
			Status:        domain.DiplomaStatusActive,
			University:    domain.University{Name: "Demo University", Code: "DEMO2026"},
			QRPayload:     stringPtr("not-a-valid-jwt"),
		},
	}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	response, err := service.Search(context.Background(), "DEMO2026", "DIP-2022-0003")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !response.Valid || response.Status != domain.DiplomaStatusActive || response.University != "Demo University" || response.VUZCode != "DEMO2026" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestSearchInvalidInput(t *testing.T) {
	service := NewVerificationService(fakeRepository{}, slog.Default(), []byte("0123456789abcdef0123456789abcdef"))

	_, err := service.Search(context.Background(), " ", " ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Search() error = %v, want ErrInvalidInput", err)
	}
}

func buildValidToken(t *testing.T, encKey []byte) string {
	t.Helper()
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	token, _ := buildTokenAndHash(t, keyPair, encKey, false)
	return token
}

func stringPtr(value string) *string {
	return &value
}

func buildTokenForKey(t *testing.T, keyPair testutil.Ed25519KeyPair, encKey []byte, mismatch bool) string {
	t.Helper()
	token, _ := buildTokenAndHash(t, keyPair, encKey, mismatch)
	return token
}

func buildTokenAndHash(t *testing.T, keyPair testutil.Ed25519KeyPair, encKey []byte, mismatch bool) (string, string) {
	t.Helper()

	payload := verifyhash.EncryptedDiplomaPayload{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		Salt:          "abcdef0123456789",
	}

	hash, err := verifyhash.HashDiplomaInput(payload.HashInput("550e8400-e29b-41d4-a716-446655440000"))
	if err != nil {
		t.Fatalf("HashDiplomaInput() error = %v", err)
	}

	outerHash := hash
	if mismatch {
		outerHash = "deadbeef"
	}

	token := testutil.MustSignJWT(t, map[string]any{
		"sub":          outerHash,
		"diploma_hash": outerHash,
		"vuz_id":       "550e8400-e29b-41d4-a716-446655440000",
		"enc":          testutil.MustEncryptA256GCM(t, encKey, payload),
		"iat":          int64(1710000000),
	}, keyPair.PrivateKey)

	return token, hash
}
