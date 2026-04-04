package config

import (
	"encoding/base64"
	"testing"
)

func TestEnvIntOrDefaultRejectsInvalidInteger(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "abc")

	_, err := envIntOrDefault("DB_MAX_CONNS", 10)
	if err == nil {
		t.Fatal("envIntOrDefault() error = nil, want non-nil")
	}
}

func TestEnvFloatOrDefaultRejectsInvalidFloat(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "abc")

	_, err := envFloatOrDefault("RATE_LIMIT_RPS", 5)
	if err == nil {
		t.Fatal("envFloatOrDefault() error = nil, want non-nil")
	}
}

func TestEnvBoolOrDefaultRejectsInvalidBool(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "not-bool")

	_, err := envBoolOrDefault("RATE_LIMIT_ENABLED", true)
	if err == nil {
		t.Fatal("envBoolOrDefault() error = nil, want non-nil")
	}
}

func TestLoadJWTEncKeyAcceptsBase64(t *testing.T) {
	t.Setenv("JWT_ENC_KEY_BASE64", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))

	key, err := loadJWTEncKey()
	if err != nil {
		t.Fatalf("loadJWTEncKey() error = %v", err)
	}

	if string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected key: %q", string(key))
	}
}

func TestLoadJWTEncKeyAcceptsLegacyHexEnvWithBase64Value(t *testing.T) {
	t.Setenv("JWT_ENC_KEY_HEX", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))

	key, err := loadJWTEncKey()
	if err != nil {
		t.Fatalf("loadJWTEncKey() error = %v", err)
	}

	if string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected key: %q", string(key))
	}
}

func TestDeriveJWTEncKeyFallsBackToSHA256(t *testing.T) {
	key, err := deriveJWTEncKey("plain-secret")
	if err != nil {
		t.Fatalf("deriveJWTEncKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Fatalf("len(key) = %d, want 32", len(key))
	}
}
