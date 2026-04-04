package testutil

import (
	"crypto/ed25519"
	"encoding/base64"
)

func CreateEd25519TestJWT(privateKey ed25519.PrivateKey, payload string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signingInput := header + "." + body

	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
