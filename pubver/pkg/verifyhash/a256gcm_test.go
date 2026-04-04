package verifyhash

import (
	"encoding/base64"
	"strings"
	"testing"

	"pubver/internal/testutil"
)

func TestDecryptEncryptedDiplomaPayload(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc := testutil.MustEncryptA256GCM(t, key, EncryptedDiplomaPayload{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		Salt:          "abcdef0123456789",
	})

	payload, err := DecryptEncryptedDiplomaPayload(enc, key)
	if err != nil {
		t.Fatalf("DecryptEncryptedDiplomaPayload() error = %v", err)
	}

	if payload.FullName != "Ivan Ivanov" || payload.Degree != "Bachelor" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDecryptEncryptedDiplomaPayloadRejectsNonStandardBase64(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc := testutil.MustEncryptA256GCM(t, key, EncryptedDiplomaPayload{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		Salt:          "abcdef0123456789",
	})

	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	urlSafe := strings.TrimRight(base64.RawURLEncoding.EncodeToString(raw), "=")

	if _, err := DecryptEncryptedDiplomaPayload(urlSafe, key); err == nil {
		t.Fatal("DecryptEncryptedDiplomaPayload() error = nil, want non-nil")
	}
}

func TestDecryptEncryptedDiplomaPayloadAcceptsStudentNameAlias(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc := testutil.MustEncryptA256GCM(t, key, map[string]any{
		"student_name":   "Ivan Ivanov",
		"diploma_number": "DVS-2024-001234",
		"specialty":      "Software Engineering",
		"degree":         "Bachelor",
		"faculty":        "FKN",
		"year":           2024,
		"salt":           "abcdef0123456789",
	})

	payload, err := DecryptEncryptedDiplomaPayload(enc, key)
	if err != nil {
		t.Fatalf("DecryptEncryptedDiplomaPayload() error = %v", err)
	}

	if payload.FullName != "Ivan Ivanov" {
		t.Fatalf("unexpected normalized full name: %+v", payload)
	}
	if payload.StudentName != "Ivan Ivanov" {
		t.Fatalf("unexpected student_name alias preservation: %+v", payload)
	}
}

func TestDecryptEncryptedDiplomaPayloadAllowsMissingDegreeAndFaculty(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc := testutil.MustEncryptA256GCM(t, key, map[string]any{
		"full_name":      "Ivan Ivanov",
		"diploma_number": "DVS-2024-001234",
		"specialty":      "Software Engineering",
		"year":           2024,
		"salt":           "abcdef0123456789",
	})

	payload, err := DecryptEncryptedDiplomaPayload(enc, key)
	if err != nil {
		t.Fatalf("DecryptEncryptedDiplomaPayload() error = %v", err)
	}

	if payload.Degree != "" || payload.Faculty != "" {
		t.Fatalf("expected empty degree and faculty, got: %+v", payload)
	}
}
