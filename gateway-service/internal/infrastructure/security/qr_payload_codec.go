package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type QRPayloadEnvelope struct {
	Subject     string
	DiplomaHash string
	VUZID       string
	Enc         string
	IssuedAt    int64
}

type QRPayloadClaims struct {
	FullName      string `json:"full_name,omitempty"`
	StudentName   string `json:"student_name,omitempty"`
	DiplomaNumber string `json:"diploma_number"`
	Specialty     string `json:"specialty"`
	Degree        string `json:"degree"`
	Faculty       string `json:"faculty"`
	Year          int    `json:"year"`
	Salt          string `json:"salt"`
}

type ParsedQRPayload struct {
	DiplomaHash string
	VUZID       string
	IssuedAt    int64
	QRPayloadClaims
}

type QRPayloadCodec struct {
	key []byte
}

func NewQRPayloadCodec(secret string) (*QRPayloadCodec, error) {
	derived, err := deriveAES256Key(secret)
	if err != nil {
		return nil, err
	}

	return &QRPayloadCodec{key: derived}, nil
}

func (c *QRPayloadCodec) Seal(claims QRPayloadClaims) (string, error) {
	claims = claims.normalized()

	plaintext, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(c.key)
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
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (c *QRPayloadCodec) Open(value string) (*QRPayloadClaims, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("encrypted qr payload is too short")
	}

	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	claims := &QRPayloadClaims{}
	if err := json.Unmarshal(plaintext, claims); err != nil {
		return nil, err
	}

	normalized := claims.normalized()
	claims = &normalized

	return claims, nil
}

func (c *QRPayloadCodec) ParseUnverifiedEnvelope(tokenString string) (*QRPayloadEnvelope, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, errors.New("qr payload is empty")
	}

	claims := jwt.MapClaims{}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	if _, _, err := parser.ParseUnverified(tokenString, claims); err != nil {
		return nil, err
	}

	envelope := &QRPayloadEnvelope{
		Subject:     claimString(claims["sub"]),
		DiplomaHash: claimString(claims["diploma_hash"]),
		VUZID:       claimString(claims["vuz_id"]),
		Enc:         claimString(claims["enc"]),
	}

	if issuedAt, err := claimInt64(claims["iat"]); err == nil {
		envelope.IssuedAt = issuedAt
	}

	if strings.TrimSpace(envelope.DiplomaHash) == "" {
		envelope.DiplomaHash = envelope.Subject
	}

	return envelope, nil
}

func (c *QRPayloadCodec) Parse(tokenString string) (*ParsedQRPayload, error) {
	envelope, err := c.ParseUnverifiedEnvelope(tokenString)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(envelope.Enc) == "" {
		return nil, errors.New("qr payload does not contain enc claim")
	}

	claims, err := c.Open(envelope.Enc)
	if err != nil {
		return nil, err
	}

	return &ParsedQRPayload{
		DiplomaHash:     envelope.DiplomaHash,
		VUZID:           envelope.VUZID,
		IssuedAt:        envelope.IssuedAt,
		QRPayloadClaims: *claims,
	}, nil
}

func deriveAES256Key(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("qr payload encryption secret must not be empty")
	}

	if decoded, ok := decodeExactKey(trimmed); ok {
		return decoded, nil
	}

	sum := sha256.Sum256(bestEffortSecretBytes(trimmed))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

func decodeExactKey(secret string) ([]byte, bool) {
	if decoded, err := hex.DecodeString(secret); err == nil && len(decoded) == 32 {
		return decoded, true
	}

	if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil && len(decoded) == 32 {
		return decoded, true
	}

	return nil, false
}

func bestEffortSecretBytes(secret string) []byte {
	if decoded, err := hex.DecodeString(secret); err == nil {
		return decoded
	}

	if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil {
		return decoded
	}

	return []byte(secret)
}

func claimString(value interface{}) string {
	if cast, ok := value.(string); ok {
		return cast
	}
	return ""
}

func claimInt64(value interface{}) (int64, error) {
	switch cast := value.(type) {
	case float64:
		return int64(cast), nil
	case int64:
		return cast, nil
	case int:
		return int64(cast), nil
	case json.Number:
		return cast.Int64()
	case string:
		return json.Number(cast).Int64()
	default:
		return 0, errors.New("unsupported numeric value")
	}
}

func (c QRPayloadClaims) normalized() QRPayloadClaims {
	c.FullName = strings.TrimSpace(c.FullName)
	c.StudentName = strings.TrimSpace(c.StudentName)

	switch {
	case c.FullName == "" && c.StudentName != "":
		c.FullName = c.StudentName
	case c.StudentName == "" && c.FullName != "":
		c.StudentName = c.FullName
	}

	return c
}
