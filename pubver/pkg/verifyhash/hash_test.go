package verifyhash

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestBuildRawDiplomaString(t *testing.T) {
	input := DiplomaHashInput{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789",
	}

	raw, err := BuildRawDiplomaString(input)
	if err != nil {
		t.Fatalf("BuildRawDiplomaString() error = %v", err)
	}

	expected := "DVS-2024-001234|Ivan Ivanov|Software Engineering|Bachelor|FKN|2024|550e8400-e29b-41d4-a716-446655440000|abcdef0123456789"
	if raw != expected {
		t.Fatalf("raw mismatch:\nwant: %s\ngot:  %s", expected, raw)
	}
}

func TestHashDiplomaInputMatchesManualSHA256(t *testing.T) {
	input := DiplomaHashInput{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789",
	}

	got, err := HashDiplomaInput(input)
	if err != nil {
		t.Fatalf("HashDiplomaInput() error = %v", err)
	}

	sum := sha256.Sum256([]byte("DVS-2024-001234|Ivan Ivanov|Software Engineering|Bachelor|FKN|2024|550e8400-e29b-41d4-a716-446655440000|abcdef0123456789"))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("hash mismatch:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestDiplomaHashInputValidateRequiresVUZID(t *testing.T) {
	input := DiplomaHashInput{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		Salt:          "abcdef0123456789",
	}

	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}

func TestBuildRawDiplomaStringAllowsEmptyDegreeAndFaculty(t *testing.T) {
	input := DiplomaHashInput{
		FullName:      "Ivan Ivanov",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "",
		Faculty:       "",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789",
	}

	raw, err := BuildRawDiplomaString(input)
	if err != nil {
		t.Fatalf("BuildRawDiplomaString() error = %v", err)
	}

	expected := "DVS-2024-001234|Ivan Ivanov|Software Engineering|||2024|550e8400-e29b-41d4-a716-446655440000|abcdef0123456789"
	if raw != expected {
		t.Fatalf("raw mismatch:\nwant: %s\ngot:  %s", expected, raw)
	}
}
