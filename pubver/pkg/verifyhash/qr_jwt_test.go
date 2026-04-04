package verifyhash

import "testing"

func TestExtractOuterQRClaimsFromMap(t *testing.T) {
	claims, err := ExtractOuterQRClaimsFromMap(map[string]any{
		"sub":          "hash",
		"diploma_hash": "hash",
		"vuz_id":       "550e8400-e29b-41d4-a716-446655440000",
		"enc":          "ZW5jcnlwdGVk",
		"iat":          1710000000,
	})
	if err != nil {
		t.Fatalf("ExtractOuterQRClaimsFromMap() error = %v", err)
	}

	if claims.VUZID != "550e8400-e29b-41d4-a716-446655440000" || claims.Enc != "ZW5jcnlwdGVk" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestExtractOuterQRClaimsFromMapFallsBackToSubWhenDiplomaHashMissing(t *testing.T) {
	claims, err := ExtractOuterQRClaimsFromMap(map[string]any{
		"sub":    "hash-from-sub",
		"vuz_id": "550e8400-e29b-41d4-a716-446655440000",
		"enc":    "ZW5jcnlwdGVk",
		"iat":    1710000000,
	})
	if err != nil {
		t.Fatalf("ExtractOuterQRClaimsFromMap() error = %v", err)
	}

	if claims.DiplomaHash != "hash-from-sub" {
		t.Fatalf("DiplomaHash = %q, want %q", claims.DiplomaHash, "hash-from-sub")
	}
}

func TestExtractInt64ClaimRejectsUnsafeFloat(t *testing.T) {
	_, err := extractInt64Claim(map[string]any{
		"iat": float64(9007199254740994),
	}, "iat")
	if err == nil {
		t.Fatal("extractInt64Claim() error = nil, want non-nil")
	}
}
