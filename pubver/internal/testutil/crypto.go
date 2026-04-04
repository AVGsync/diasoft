package testutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

type Ed25519KeyPair struct {
	PublicKey    ed25519.PublicKey
	PrivateKey   ed25519.PrivateKey
	PublicKeyHex string
}

func MustGenerateEd25519KeyPair(t *testing.T) Ed25519KeyPair {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key pair: %v", err)
	}

	return Ed25519KeyPair{
		PublicKey:    publicKey,
		PrivateKey:   privateKey,
		PublicKeyHex: hex.EncodeToString(publicKey),
	}
}

func MustEncryptA256GCM(t *testing.T, key []byte, payload any) string {
	t.Helper()

	plaintext, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal encrypted payload: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("init aes cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("init gcm: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	raw := append(append([]byte{}, nonce...), ciphertext...)
	return base64.StdEncoding.EncodeToString(raw)
}

func MustSignJWT(t *testing.T, claims map[string]any, privateKey ed25519.PrivateKey) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal jwt header: %v", err)
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := encodedHeader + "." + encodedPayload
	signature := ed25519.Sign(privateKey, []byte(signingInput))

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}
