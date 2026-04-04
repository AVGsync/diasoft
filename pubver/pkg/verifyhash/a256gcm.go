package verifyhash

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type EncryptedDiplomaPayload struct {
	FullName      string `json:"full_name"`
	StudentName   string `json:"student_name,omitempty"`
	DiplomaNumber string `json:"diploma_number"`
	Specialty     string `json:"specialty"`
	Degree        string `json:"degree"`
	Faculty       string `json:"faculty"`
	Year          int    `json:"year"`
	Salt          string `json:"salt"`
}

func (p *EncryptedDiplomaPayload) normalize() error {
	p.FullName = strings.TrimSpace(p.FullName)
	p.StudentName = strings.TrimSpace(p.StudentName)

	switch {
	case p.FullName == "" && p.StudentName != "":
		p.FullName = p.StudentName
	case p.FullName != "" && p.StudentName != "" && p.FullName != p.StudentName:
		return fmt.Errorf("full_name and student_name must match when both are present")
	}

	return nil
}

func (p EncryptedDiplomaPayload) Validate() error {
	if err := validateDiplomaCoreFields(
		p.FullName,
		p.DiplomaNumber,
		p.Specialty,
		p.Year,
	); err != nil {
		return err
	}

	if strings.TrimSpace(p.Salt) == "" {
		return fmt.Errorf("salt must not be empty")
	}

	return nil
}

func (p EncryptedDiplomaPayload) HashInput(vuzID string) DiplomaHashInput {
	return DiplomaHashInput{
		FullName:      p.FullName,
		DiplomaNumber: p.DiplomaNumber,
		Specialty:     p.Specialty,
		Degree:        p.Degree,
		Faculty:       p.Faculty,
		Year:          p.Year,
		VUZID:         vuzID,
		Salt:          p.Salt,
	}
}

func DecryptEncryptedDiplomaPayload(encValue string, key []byte) (EncryptedDiplomaPayload, error) {
	rawCiphertext, err := decodeEncryptedValue(encValue)
	if err != nil {
		return EncryptedDiplomaPayload{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedDiplomaPayload{}, fmt.Errorf("init aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedDiplomaPayload{}, fmt.Errorf("init aes gcm: %w", err)
	}

	minSize := gcm.NonceSize() + gcm.Overhead()
	if len(rawCiphertext) < minSize {
		return EncryptedDiplomaPayload{}, fmt.Errorf("enc must contain nonce and ciphertext")
	}

	nonce := rawCiphertext[:gcm.NonceSize()]
	ciphertext := rawCiphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return EncryptedDiplomaPayload{}, fmt.Errorf("decrypt enc with A256GCM: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(plaintext)))
	decoder.DisallowUnknownFields()

	var payload EncryptedDiplomaPayload
	if err := decoder.Decode(&payload); err != nil {
		return EncryptedDiplomaPayload{}, fmt.Errorf("decode decrypted enc payload: %w", err)
	}

	if err := payload.normalize(); err != nil {
		return EncryptedDiplomaPayload{}, err
	}

	if err := payload.Validate(); err != nil {
		return EncryptedDiplomaPayload{}, err
	}

	return payload, nil
}

func decodeEncryptedValue(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("enc must not be empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode enc base64: %w", err)
	}

	return decoded, nil
}
