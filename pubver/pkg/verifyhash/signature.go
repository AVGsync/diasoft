package verifyhash

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
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

func VerifyRS256JWT(token, publicKeyValue string) error {
	header, signingInput, signature, err := decodeJWTForVerification(token)
	if err != nil {
		return err
	}

	if header.Alg != "RS256" {
		return fmt.Errorf("unsupported jwt alg %q", header.Alg)
	}

	publicKey, err := parseRSAPublicKey(publicKeyValue)
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("verify rs256 jwt: %w", err)
	}

	return nil
}

func parseRSAPublicKey(value string) (*rsa.PublicKey, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		return parseRSAPublicKeyBytes(block.Bytes)
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return parseRSAPublicKeyBytes(decoded)
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
		return parseRSAPublicKeyBytes(decoded)
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil {
		return parseRSAPublicKeyBytes(decoded)
	}

	return nil, fmt.Errorf("unsupported rsa public key format")
}

func parseRSAPublicKeyBytes(der []byte) (*rsa.PublicKey, error) {
	if parsed, err := x509.ParsePKIXPublicKey(der); err == nil {
		publicKey, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key must be RSA")
		}
		return publicKey, nil
	}

	if parsed, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return parsed, nil
	}

	return nil, fmt.Errorf("failed to parse rsa public key")
}
