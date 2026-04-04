package verifyhash

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
)

type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

func VerifyEd25519JWT(token, publicKeyValue string) error {
	header, signingInput, signature, err := decodeJWTForVerification(token)
	if err != nil {
		return err
	}

	if header.Alg != "EdDSA" {
		return fmt.Errorf("unsupported jwt alg %q", header.Alg)
	}

	publicKey, err := parseEd25519PublicKey(publicKeyValue)
	if err != nil {
		return err
	}

	if !ed25519.Verify(publicKey, []byte(signingInput), signature) {
		return fmt.Errorf("verify ed25519 jwt: signature is invalid")
	}

	return nil
}

func parseEd25519PublicKey(value string) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		return parseEd25519PublicKeyBytes(block.Bytes)
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return parseEd25519PublicKeyBytes(decoded)
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
		return parseEd25519PublicKeyBytes(decoded)
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil {
		return parseEd25519PublicKeyBytes(decoded)
	}

	return nil, fmt.Errorf("unsupported ed25519 public key format")
}

func parseEd25519PublicKeyBytes(raw []byte) (ed25519.PublicKey, error) {
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}

	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ed25519 public key: %w", err)
	}

	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key must be Ed25519")
	}

	return publicKey, nil
}
