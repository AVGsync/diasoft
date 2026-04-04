package verifyhash

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"pubver/internal/testutil"
)

func TestVerifyEd25519JWT(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 keypair: %v", err)
	}

	payload := `{"sub":"abc123","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","degree":"Bachelor","faculty":"FKN","year":2024,"salt":"pepper"}`
	token, err := testutil.CreateEd25519TestJWT(privateKey, payload)
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	if err := VerifyEd25519JWT(token, string(publicKeyPEM)); err != nil {
		t.Fatalf("verify ed25519 jwt: %v", err)
	}
}

func TestParseEd25519PublicKeyBase64DER(t *testing.T) {
	t.Parallel()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 keypair: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	if _, err := parseEd25519PublicKey(base64.StdEncoding.EncodeToString(publicKeyDER)); err != nil {
		t.Fatalf("parse ed25519 public key from base64: %v", err)
	}
}

func TestVerifyEd25519JWTRejectsBrokenSignature(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 keypair: %v", err)
	}

	payload := `{"sub":"abc123","vuz_id":"550e8400-e29b-41d4-a716-446655440000","diploma_number":"DVS-2024-001234","student_name":"Ivanov Ivan Ivanovich","specialty":"Software Engineering","degree":"Bachelor","faculty":"FKN","year":2024,"salt":"pepper"}`
	token, err := testutil.CreateEd25519TestJWT(privateKey, payload)
	if err != nil {
		t.Fatalf("create test jwt: %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	broken := token[:len(token)-2] + "ab"
	if err := VerifyEd25519JWT(broken, string(publicKeyPEM)); err == nil {
		t.Fatalf("expected broken jwt signature to fail")
	}
}
