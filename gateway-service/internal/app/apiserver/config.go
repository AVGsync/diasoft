package apiserver

import (
	"time"

	kafkainfra "github.com/diasoft/gateway-service/internal/infrastructure/kafka"
	"github.com/diasoft/gateway-service/internal/repository/postgres"
)

type Config struct {
	BindAddr                  string             `toml:"bind_addr"`
	LogLevel                  string             `toml:"log_level"`
	PublicBaseURL             string             `toml:"public_base_url"`
	JWTSecret                 string             `toml:"jwt_secret"`
	ShareJWTSecret            string             `toml:"share_jwt_secret"`
	SigningKeysMasterKey      string             `toml:"signing_keys_master_key"`
	QRPayloadEncryptionSecret string             `toml:"qr_payload_encryption_secret"`
	AccessTokenTTL            time.Duration      `toml:"access_token_ttl"`
	ShareTokenTTL             time.Duration      `toml:"share_token_ttl"`
	BootstrapAdminEmail       string             `toml:"bootstrap_admin_email"`
	BootstrapAdminPassword    string             `toml:"bootstrap_admin_password"`
	DB                        *postgres.Config   `toml:"db"`
	Kafka                     *kafkainfra.Config `toml:"kafka"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr:                  ":8080",
		LogLevel:                  "debug",
		PublicBaseURL:             "http://localhost:8080",
		JWTSecret:                 "gateway-access-secret",
		ShareJWTSecret:            "gateway-share-secret",
		SigningKeysMasterKey:      "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		QRPayloadEncryptionSecret: "XpMcu0eI/4SrmsO99dYLUPAwgyarZbrS92RzFmUjTPI=",
		AccessTokenTTL:            24 * time.Hour,
		ShareTokenTTL:             72 * time.Hour,
		BootstrapAdminEmail:       "admin@platform.local",
		BootstrapAdminPassword:    "admin12345",
		DB:                        postgres.NewConfig(),
		Kafka:                     kafkainfra.NewConfig(),
	}
}
