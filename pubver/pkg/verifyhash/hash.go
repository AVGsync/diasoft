package verifyhash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type DiplomaHashInput struct {
	FullName      string
	DiplomaNumber string
	Specialty     string
	Degree        string
	Faculty       string
	Year          int
	VUZID         string
	Salt          string
}

func validateDiplomaCoreFields(fullName, diplomaNumber, specialty string, year int) error {
	switch {
	case strings.TrimSpace(fullName) == "":
		return fmt.Errorf("full_name must not be empty")
	case strings.TrimSpace(diplomaNumber) == "":
		return fmt.Errorf("diploma_number must not be empty")
	case strings.TrimSpace(specialty) == "":
		return fmt.Errorf("specialty must not be empty")
	case year <= 0:
		return fmt.Errorf("year must be a positive integer")
	default:
		return nil
	}
}

func (i DiplomaHashInput) Validate() error {
	if err := validateDiplomaCoreFields(
		i.FullName,
		i.DiplomaNumber,
		i.Specialty,
		i.Year,
	); err != nil {
		return err
	}

	switch {
	case strings.TrimSpace(i.VUZID) == "":
		return fmt.Errorf("vuz_id must not be empty")
	case strings.TrimSpace(i.Salt) == "":
		return fmt.Errorf("salt must not be empty")
	default:
		return nil
	}
}

func HashDiplomaInput(input DiplomaHashInput) (string, error) {
	raw, err := BuildRawDiplomaString(input)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

func BuildRawDiplomaString(input DiplomaHashInput) (string, error) {
	if err := input.Validate(); err != nil {
		return "", err
	}

	parts := []string{
		strings.TrimSpace(input.DiplomaNumber),
		strings.TrimSpace(input.FullName),
		strings.TrimSpace(input.Specialty),
		strings.TrimSpace(input.Degree),
		strings.TrimSpace(input.Faculty),
		strconv.Itoa(input.Year),
		strings.TrimSpace(input.VUZID),
		strings.TrimSpace(input.Salt),
	}

	return strings.Join(parts, "|"), nil
}
