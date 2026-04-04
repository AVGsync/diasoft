package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const sealedPayloadPrefix = "v1:"

type KeyEncryptor struct {
	key []byte
}

func NewKeyEncryptor(base64Key string) (*KeyEncryptor, error) {
	decoded, err := deriveAES256Key(base64Key)
	if err != nil {
		return nil, err
	}

	return &KeyEncryptor{key: decoded}, nil
}

func (e *KeyEncryptor) Seal(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, ciphertext...)

	return sealedPayloadPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func (e *KeyEncryptor) Open(value string) ([]byte, error) {
	if !strings.HasPrefix(value, sealedPayloadPrefix) {
		return nil, errors.New("unsupported sealed payload version")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, sealedPayloadPrefix))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("sealed payload is too short")
	}

	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
