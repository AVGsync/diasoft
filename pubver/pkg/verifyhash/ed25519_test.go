package verifyhash

import (
	"encoding/hex"
	"testing"

	"pubver/internal/testutil"
)

func TestVerifyEd25519JWTWithHexPublicKey(t *testing.T) {
	keyPair := testutil.MustGenerateEd25519KeyPair(t)
	token := testutil.MustSignJWT(t, map[string]any{
		"sub":          "hash",
		"diploma_hash": "hash",
		"vuz_id":       "550e8400-e29b-41d4-a716-446655440000",
		"enc":          "ZW5jcnlwdGVk",
		"iat":          int64(1710000000),
	}, keyPair.PrivateKey)

	if err := VerifyEd25519JWT(token, hex.EncodeToString(keyPair.PublicKey)); err != nil {
		t.Fatalf("VerifyEd25519JWT() error = %v", err)
	}
}
