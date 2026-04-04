package verifyhash

import (
	"encoding/base64"
	"testing"
)

func TestExtractQRClaims(t *testing.T) {
	t.Parallel()

	payload := `{"sub":"abc123","diploma_hash":"abc123","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","degree":"Bachelor","faculty":"FKN","year":2024,"salt":"pepper"}`
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "."

	claims, err := ExtractQRClaims(token)
	if err != nil {
		t.Fatalf("extract qr claims: %v", err)
	}

	if claims.DiplomaNumber != "DVS-2024-001234" {
		t.Fatalf("unexpected diploma_number: %v", claims.DiplomaNumber)
	}
	if claims.StudentName != "Ivanov Ivan Ivanovich" {
		t.Fatalf("unexpected student_name: %v", claims.StudentName)
	}
	if claims.Degree != "Bachelor" {
		t.Fatalf("unexpected degree: %v", claims.Degree)
	}
	if claims.Faculty != "FKN" {
		t.Fatalf("unexpected faculty: %v", claims.Faculty)
	}
	if claims.Year != 2024 {
		t.Fatalf("unexpected year: %d", claims.Year)
	}
	if claims.Salt != "pepper" {
		t.Fatalf("unexpected salt: %s", claims.Salt)
	}
}

func TestExtractVUZID(t *testing.T) {
	t.Parallel()

	payload := `{"vuz_id":"550e8400-e29b-41d4-a716-446655440000"}`
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "."

	vuzID, err := ExtractVUZID(token)
	if err != nil {
		t.Fatalf("extract vuz_id: %v", err)
	}

	if vuzID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected vuz_id: %s", vuzID)
	}
}

func TestExtractQRClaimsFromMap(t *testing.T) {
	t.Parallel()

	claims, err := ExtractQRClaimsFromMap(map[string]any{
		"sub":            "abc123",
		"diploma_hash":   "abc123",
		"vuz_id":         "550e8400-e29b-41d4-a716-446655440000",
		"diploma_number": "DVS-2024-001234",
		"student_name":   "Ivanov Ivan Ivanovich",
		"specialty":      "Software Engineering",
		"degree":         "Bachelor",
		"faculty":        "FKN",
		"year":           2024,
		"salt":           "pepper",
	})
	if err != nil {
		t.Fatalf("extract qr claims from map: %v", err)
	}

	if claims.VUZID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected vuz_id: %s", claims.VUZID)
	}
	if claims.DiplomaNumber != "DVS-2024-001234" {
		t.Fatalf("unexpected diploma_number: %v", claims.DiplomaNumber)
	}
	if claims.Degree != "Bachelor" {
		t.Fatalf("unexpected degree: %v", claims.Degree)
	}
	if claims.Faculty != "FKN" {
		t.Fatalf("unexpected faculty: %v", claims.Faculty)
	}
}

func TestExtractVUZIDFromMap(t *testing.T) {
	t.Parallel()

	vuzID, err := ExtractVUZIDFromMap(map[string]any{
		"vuz_id": "550e8400-e29b-41d4-a716-446655440000",
	})
	if err != nil {
		t.Fatalf("extract vuz_id from map: %v", err)
	}

	if vuzID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected vuz_id: %s", vuzID)
	}
}
