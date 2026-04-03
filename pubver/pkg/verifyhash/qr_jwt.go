package verifyhash

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

type QRClaims struct {
	Sub           string
	DiplomaHash   string
	VUZID         string
	DiplomaNumber string
	StudentName   string
	Specialty     string
	Year          int
	Salt          string
}

func (c QRClaims) HashInput() DiplomaHashInput {
	return DiplomaHashInput{
		FullName:      c.StudentName,
		DiplomaNumber: c.DiplomaNumber,
		Specialty:     c.Specialty,
		Year:          c.Year,
		VUZID:         c.VUZID,
		Salt:          c.Salt,
	}
}

func ExtractQRClaims(token string) (QRClaims, error) {
	claims, err := DecodeUnverifiedJWT(token)
	if err != nil {
		return QRClaims{}, err
	}

	return ExtractQRClaimsFromMap(claims)
}

func ExtractQRClaimsFromMap(claims map[string]any) (QRClaims, error) {
	vuzID, err := ExtractVUZIDFromMap(claims)
	if err != nil {
		return QRClaims{}, err
	}

	year, err := extractIntClaim(claims, "year")
	if err != nil {
		return QRClaims{}, err
	}

	studentName, err := extractStringClaim(claims, "student_name")
	if err != nil {
		fullName, fallbackErr := extractStringClaim(claims, "full_name")
		if fallbackErr != nil {
			return QRClaims{}, err
		}
		studentName = fullName
	}

	diplomaNumber, err := extractStringClaim(claims, "diploma_number")
	if err != nil {
		return QRClaims{}, err
	}
	specialty, err := extractStringClaim(claims, "specialty")
	if err != nil {
		return QRClaims{}, err
	}
	salt, err := extractStringClaim(claims, "salt")
	if err != nil {
		return QRClaims{}, err
	}

	return QRClaims{
		Sub:           optionalStringClaim(claims, "sub"),
		DiplomaHash:   optionalStringClaim(claims, "diploma_hash"),
		VUZID:         vuzID,
		DiplomaNumber: diplomaNumber,
		StudentName:   studentName,
		Specialty:     specialty,
		Year:          year,
		Salt:          salt,
	}, nil
}

func ExtractVUZID(token string) (string, error) {
	claims, err := DecodeUnverifiedJWT(token)
	if err != nil {
		return "", err
	}

	return ExtractVUZIDFromMap(claims)
}

func ExtractVUZIDFromMap(claims map[string]any) (string, error) {
	return extractStringClaim(claims, "vuz_id")
}

func DecodeUnverifiedJWTHeader(token string) (JWTHeader, error) {
	parts, err := splitJWT(token)
	if err != nil {
		return JWTHeader{}, err
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return JWTHeader{}, fmt.Errorf("decode jwt header: %w", err)
	}

	var header JWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return JWTHeader{}, fmt.Errorf("unmarshal jwt header: %w", err)
	}

	return header, nil
}

// DecodeUnverifiedJWT decodes the payload of a JWT without verifying its signature.
func DecodeUnverifiedJWT(token string) (map[string]any, error) {
	parts, err := splitJWT(token)
	if err != nil {
		return nil, err
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode jwt payload: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()

	var claims map[string]any
	if err := decoder.Decode(&claims); err != nil {
		return nil, fmt.Errorf("unmarshal jwt payload: %w", err)
	}

	return claims, nil
}

func decodeJWTForVerification(token string) (JWTHeader, string, []byte, error) {
	parts, err := splitJWT(token)
	if err != nil {
		return JWTHeader{}, "", nil, err
	}

	header, err := DecodeUnverifiedJWTHeader(token)
	if err != nil {
		return JWTHeader{}, "", nil, err
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return JWTHeader{}, "", nil, fmt.Errorf("decode jwt signature: %w", err)
	}

	return header, parts[0] + "." + parts[1], signature, nil
}

func splitJWT(token string) ([3]string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return [3]string{}, errors.New("token must have 3 segments")
	}

	return [3]string{parts[0], parts[1], parts[2]}, nil
}

func optionalStringClaim(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}

func extractStringClaim(claims map[string]any, key string) (string, error) {
	rawValue, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("claim %q is missing", key)
	}

	value, ok := rawValue.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("claim %q must be a non-empty string", key)
	}

	return strings.TrimSpace(value), nil
}

func extractIntClaim(claims map[string]any, key string) (int, error) {
	rawValue, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf("claim %q is missing", key)
	}

	switch typed := rawValue.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("claim %q must be an integer", key)
		}
		return int(parsed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("claim %q must be an integer", key)
		}
		return int(typed), nil
	case int:
		return typed, nil
	default:
		return 0, fmt.Errorf("claim %q must be an integer", key)
	}
}
