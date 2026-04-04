package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"pubver/internal/domain"
	"pubver/internal/testutil"
	"pubver/pkg/verifyhash"
)

type fakeRepository struct {
	verificationKeys map[string]*domain.UniversityVerificationKey
	byHash           map[string]*domain.DiplomaRecord
	byDiploma        map[string]*domain.DiplomaRecord
	findKeyErr       error
	findByHashErr    error
	findByDiplomaErr error
}

func (r *fakeRepository) FindUniversityVerificationKeyByID(_ context.Context, vuzID string) (*domain.UniversityVerificationKey, error) {
	if r.findKeyErr != nil {
		return nil, r.findKeyErr
	}

	verificationKey := r.verificationKeys[vuzID]
	if verificationKey == nil {
		return nil, domain.ErrUniversityVerificationKeyNotFound
	}

	return verificationKey, nil
}

func (r *fakeRepository) FindByHash(_ context.Context, hash string) (*domain.DiplomaRecord, error) {
	if r.findByHashErr != nil {
		return nil, r.findByHashErr
	}

	return r.byHash[hash], nil
}

func (r *fakeRepository) FindByDiplomaNumber(_ context.Context, vuzCode, diplomaNumber string) (*domain.DiplomaRecord, error) {
	if r.findByDiplomaErr != nil {
		return nil, r.findByDiplomaErr
	}

	return r.byDiploma[vuzCode+"::"+diplomaNumber], nil
}

type verifyTokenFixture struct {
	token         string
	hash          string
	vuzID         string
	publicKeyPEM  string
	diplomaNumber string
	university    string
	vuzCode       string
}

func TestVerifyPayloadReturnsActiveDiploma(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
		byHash: map[string]*domain.DiplomaRecord{
			fixture.hash: buildRecord(fixture, domain.DiplomaStatusActive),
		},
	})

	response, err := service.VerifyPayload(context.Background(), fixture.token)
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

func TestVerifyPayloadReturnsRevokedDiploma(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
		byHash: map[string]*domain.DiplomaRecord{
			fixture.hash: buildRecord(fixture, domain.DiplomaStatusRevoked),
		},
	})

	response, err := service.VerifyPayload(context.Background(), fixture.token)
	if err != nil {
		t.Fatalf("verify payload: %v", err)
	}

	if response.Valid {
		t.Fatalf("expected revoked diploma to be invalid")
	}

	if response.Status != domain.DiplomaStatusRevoked {
		t.Fatalf("unexpected status: %s", response.Status)
	}

	if response.RevokedAt == nil {
		t.Fatalf("expected revoked_at to be set")
	}
}

func TestVerifyPayloadReturnsNotFoundWhenDiplomaMissing(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
		byHash: map[string]*domain.DiplomaRecord{},
	})

	response, err := service.VerifyPayload(context.Background(), fixture.token)
	if err != nil {
		t.Fatalf("verify payload: %v", err)
	}

	if response.Valid {
		t.Fatalf("expected not found diploma to be invalid")
	}

	if response.Status != domain.DiplomaStatusNotFound {
		t.Fatalf("unexpected status: %s", response.Status)
	}

	if response.Hash != fixture.hash {
		t.Fatalf("unexpected response hash: got %q want %q", response.Hash, fixture.hash)
	}
}

func TestVerifyPayloadRejectsWhitespaceToken(t *testing.T) {
	t.Parallel()

	service := newTestVerificationService(&fakeRepository{})

	_, err := service.VerifyPayload(context.Background(), "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestVerifyPayloadRejectsMalformedJWT(t *testing.T) {
	t.Parallel()

	service := newTestVerificationService(&fakeRepository{})

	_, err := service.VerifyPayload(context.Background(), "broken")
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
}

func TestVerifyPayloadMissingUniversityReturnsGenericInvalidPayload(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{},
	})

	_, err := service.VerifyPayload(context.Background(), fixture.token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
}

func TestVerifyPayloadRejectsWrongSignature(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	otherPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate second ed25519 keypair: %v", err)
	}

	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: marshalPublicKeyPEM(t, otherPublicKey)},
		},
	})

	_, err = service.VerifyPayload(context.Background(), fixture.token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
}

func TestVerifyPayloadRejectsDiplomaHashMismatch(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, func(payload map[string]any) {
		payload["diploma_hash"] = "mismatch"
	})
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
	})

	_, err := service.VerifyPayload(context.Background(), fixture.token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
	if !errors.Is(err, domain.ErrDiplomaHashClaimMismatch) {
		t.Fatalf("expected diploma_hash mismatch error, got %v", err)
	}
}

func TestVerifyPayloadRejectsSubMismatch(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, func(payload map[string]any) {
		payload["sub"] = "mismatch"
	})
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
	})

	_, err := service.VerifyPayload(context.Background(), fixture.token)
	if !errors.Is(err, domain.ErrInvalidPayload) {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
	if !errors.Is(err, domain.ErrSubClaimMismatch) {
		t.Fatalf("expected sub mismatch error, got %v", err)
	}
}

func TestVerifyPayloadAllowsLookupWithoutOptionalHashClaims(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, func(payload map[string]any) {
		delete(payload, "sub")
		delete(payload, "diploma_hash")
	})
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
		byHash: map[string]*domain.DiplomaRecord{
			fixture.hash: buildRecord(fixture, domain.DiplomaStatusActive),
		},
	})

	response, err := service.VerifyPayload(context.Background(), fixture.token)
	if err != nil {
		t.Fatalf("verify payload: %v", err)
	}

	if !response.Valid || response.Status != domain.DiplomaStatusActive {
		t.Fatalf("unexpected verify response: %+v", response)
	}
}

func TestVerifyPayloadReturnsRepositoryErrorWhenFindingHash(t *testing.T) {
	t.Parallel()

	fixture := setupTestToken(t, nil)
	repoErr := errors.New("database unavailable")
	service := newTestVerificationService(&fakeRepository{
		verificationKeys: map[string]*domain.UniversityVerificationKey{
			fixture.vuzID: {PublicKey: fixture.publicKeyPEM},
		},
		findByHashErr: repoErr,
	})

	_, err := service.VerifyPayload(context.Background(), fixture.token)
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestSearchReturnsHappyPath(t *testing.T) {
	t.Parallel()

	year := 2024
	specialty := "Software Engineering"
	record := &domain.DiplomaRecord{
		Hash:          "hash",
		DiplomaNumber: "DVS-2024-001234",
		Status:        domain.DiplomaStatusActive,
		University: domain.University{
			Code: "001X7276",
			Name: "Bauman Moscow State Technical University",
		},
		GraduateYear: &year,
		Specialty:    &specialty,
	}

	service := newTestVerificationService(&fakeRepository{
		byDiploma: map[string]*domain.DiplomaRecord{
			"001X7276::DVS-2024-001234": record,
		},
	})

	response, err := service.Search(context.Background(), " 001X7276 ", " DVS-2024-001234 ")
	if err != nil {
		t.Fatalf("search diploma: %v", err)
	}

	if !response.Valid || response.Status != domain.DiplomaStatusActive {
		t.Fatalf("unexpected search response: %+v", response)
	}
	if response.University != record.University.Name {
		t.Fatalf("unexpected university: got %q want %q", response.University, record.University.Name)
	}
	if response.VUZCode != record.University.Code {
		t.Fatalf("unexpected vuz code: got %q want %q", response.VUZCode, record.University.Code)
	}
	if response.Year == nil || *response.Year != year {
		t.Fatalf("unexpected year: %+v", response.Year)
	}
	if response.Specialty == nil || *response.Specialty != specialty {
		t.Fatalf("unexpected specialty: %+v", response.Specialty)
	}
}

func TestSearchReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := newTestVerificationService(&fakeRepository{
		byDiploma: map[string]*domain.DiplomaRecord{},
	})

	response, err := service.Search(context.Background(), "001X7276", "AB-42")
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

func TestSearchRejectsEmptyParameters(t *testing.T) {
	t.Parallel()

	service := newTestVerificationService(&fakeRepository{})

	_, err := service.Search(context.Background(), " ", "\t")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestSearchReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("database unavailable")
	service := newTestVerificationService(&fakeRepository{
		findByDiplomaErr: repoErr,
	})

	_, err := service.Search(context.Background(), "001X7276", "DVS-2024-001234")
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func newTestVerificationService(repo *fakeRepository) *VerificationService {
	return NewVerificationService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func setupTestToken(t *testing.T, mutate func(map[string]any)) verifyTokenFixture {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 keypair: %v", err)
	}

	input := verifyhash.DiplomaHashInput{
		FullName:      "Ivanov Ivan Ivanovich",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "pepper",
	}

	hash, err := verifyhash.HashDiplomaInput(input)
	if err != nil {
		t.Fatalf("hash diploma input: %v", err)
	}

	payload := map[string]any{
		"sub":            hash,
		"diploma_hash":   hash,
		"vuz_id":         input.VUZID,
		"diploma_number": input.DiplomaNumber,
		"student_name":   input.FullName,
		"specialty":      input.Specialty,
		"degree":         "Bachelor",
		"faculty":        "FKN",
		"year":           input.Year,
		"salt":           input.Salt,
	}

	if mutate != nil {
		mutate(payload)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	token, err := testutil.CreateEd25519TestJWT(privateKey, string(payloadBytes))
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	return verifyTokenFixture{
		token:         token,
		hash:          hash,
		vuzID:         input.VUZID,
		publicKeyPEM:  marshalPublicKeyPEM(t, publicKey),
		diplomaNumber: input.DiplomaNumber,
		university:    "Bauman Moscow State Technical University",
		vuzCode:       "001X7276",
	}
}

func marshalPublicKeyPEM(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER}))
}

func buildRecord(fixture verifyTokenFixture, status domain.DiplomaStatus) *domain.DiplomaRecord {
	record := &domain.DiplomaRecord{
		Hash:          fixture.hash,
		DiplomaNumber: fixture.diplomaNumber,
		Status:        status,
		University: domain.University{
			Code: fixture.vuzCode,
			Name: fixture.university,
		},
	}

	if status == domain.DiplomaStatusRevoked {
		revokedAt := time.Date(2025, time.January, 15, 10, 0, 0, 0, time.UTC)
		record.RevokedAt = &revokedAt
	}

	return record
}
