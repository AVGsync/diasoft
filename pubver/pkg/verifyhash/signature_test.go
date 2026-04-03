package verifyhash

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"pubver/internal/testutil"
)

func TestVerifyRS256JWT(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa keypair: %v", err)
	}

	payload := `{"sub":"abc123","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","year":2024,"salt":"pepper"}`
	token, err := testutil.CreateRS256TestJWT(privateKey, payload)
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	if err := VerifyRS256JWT(token, string(publicKeyPEM)); err != nil {
		t.Fatalf("verify rs256 jwt: %v", err)
	}
}

func TestParseRSAPublicKeyBase64DER(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa keypair: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	if _, err := parseRSAPublicKey(base64.StdEncoding.EncodeToString(publicKeyDER)); err != nil {
		t.Fatalf("parse rsa public key from base64: %v", err)
	}
}

func TestVerifyRS256JWTRejectsBrokenSignature(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa keypair: %v", err)
	}

	payload := `{"sub":"abc123","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","year":2024,"salt":"pepper"}`
	token, err := testutil.CreateRS256TestJWT(privateKey, payload)
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	broken := token[:len(token)-2] + "ab"
	if err := VerifyRS256JWT(broken, string(publicKeyPEM)); err == nil {
		t.Fatalf("expected broken jwt signature to fail")
	}
}
