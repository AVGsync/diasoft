package apiserver

import (
	"encoding/base64"
	"testing"
)

func TestValidateAcceptsRequiredProductionConfig(t *testing.T) {
	config := NewConfig()
	config.DB.DatabaseURL = "postgres://gateway_user:gateway_password@localhost:5432/postgres?sslmode=disable"
	config.JWTSecret = "jwt-secret"
	config.ShareJWTSecret = "share-secret"
	config.SigningKeysMasterKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	config.QRPayloadEncryptionSecret = base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	config.Kafka.Brokers = []string{"localhost:9092"}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidBase64Secrets(t *testing.T) {
	config := NewConfig()
	config.DB.DatabaseURL = "postgres://gateway_user:gateway_password@localhost:5432/postgres?sslmode=disable"
	config.JWTSecret = "jwt-secret"
	config.ShareJWTSecret = "share-secret"
	config.SigningKeysMasterKey = "plain-secret"
	config.QRPayloadEncryptionSecret = base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	config.Kafka.Brokers = []string{"localhost:9092"}

	if err := config.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}

func TestValidateRejectsIncompleteBootstrapAdmin(t *testing.T) {
	config := NewConfig()
	config.DB.DatabaseURL = "postgres://gateway_user:gateway_password@localhost:5432/postgres?sslmode=disable"
	config.JWTSecret = "jwt-secret"
	config.ShareJWTSecret = "share-secret"
	config.SigningKeysMasterKey = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	config.QRPayloadEncryptionSecret = base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	config.BootstrapAdminEmail = "admin@example.com"
	config.Kafka.Brokers = []string{"localhost:9092"}

	if err := config.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}
