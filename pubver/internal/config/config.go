package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	RequestTimeout time.Duration
	LogLevel       string
	DBMaxConns     int32
	JWTEncKey      []byte
	RateLimit      RateLimitConfig
}

type RateLimitConfig struct {
	Enabled         bool
	RequestsPerSec  float64
	Burst           int
	VisitorTTL      time.Duration
	CleanupInterval time.Duration
}

func Load() (Config, error) {
	dbMaxConns, err := envIntOrDefault("DB_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	rateLimitEnabled, err := envBoolOrDefault("RATE_LIMIT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	rateLimitBurst, err := envIntOrDefault("RATE_LIMIT_BURST", 20)
	if err != nil {
		return Config{}, err
	}
	rateLimitRPS, err := envFloatOrDefault("RATE_LIMIT_RPS", 5)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    envOrDefault("LOG_LEVEL", "info"),
		DBMaxConns:  int32(dbMaxConns),
		RateLimit: RateLimitConfig{
			Enabled:        rateLimitEnabled,
			RequestsPerSec: rateLimitRPS,
			Burst:          rateLimitBurst,
		},
	}

	requestTimeout, err := time.ParseDuration(envOrDefault("REQUEST_TIMEOUT", "5s"))
	if err != nil {
		return Config{}, err
	}
	cfg.RequestTimeout = requestTimeout

	rateLimitVisitorTTL, err := time.ParseDuration(envOrDefault("RATE_LIMIT_VISITOR_TTL", "10m"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_VISITOR_TTL must be a valid duration: %w", err)
	}
	cfg.RateLimit.VisitorTTL = rateLimitVisitorTTL

	rateLimitCleanupInterval, err := time.ParseDuration(envOrDefault("RATE_LIMIT_CLEANUP_INTERVAL", "5m"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_CLEANUP_INTERVAL must be a valid duration: %w", err)
	}
	cfg.RateLimit.CleanupInterval = rateLimitCleanupInterval

	if cfg.RateLimit.Enabled {
		switch {
		case cfg.RateLimit.RequestsPerSec <= 0:
			return Config{}, errors.New("RATE_LIMIT_RPS must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.Burst <= 0:
			return Config{}, errors.New("RATE_LIMIT_BURST must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.VisitorTTL <= 0:
			return Config{}, errors.New("RATE_LIMIT_VISITOR_TTL must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.CleanupInterval <= 0:
			return Config{}, errors.New("RATE_LIMIT_CLEANUP_INTERVAL must be greater than zero when rate limiting is enabled")
		}
	}

	cfg.DatabaseURL, err = loadDatabaseURL()
	if err != nil {
		return Config{}, err
	}

	cfg.JWTEncKey, err = loadJWTEncKey()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadDatabaseURL() (string, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL != "" {
		return databaseURL, nil
	}

	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	database := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	if host == "" || database == "" || user == "" {
		return "", errors.New("set DATABASE_URL or configure POSTGRES_HOST, POSTGRES_DB and POSTGRES_USER")
	}

	port := strings.TrimSpace(envOrDefault("POSTGRES_PORT", "5432"))
	password := os.Getenv("POSTGRES_PASSWORD")
	sslMode := strings.TrimSpace(envOrDefault("POSTGRES_SSLMODE", "disable"))

	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   database,
	}

	query := connectionURL.Query()
	query.Set("sslmode", sslMode)
	connectionURL.RawQuery = query.Encode()

	return connectionURL.String(), nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func envIntOrDefault(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return parsed, nil
}

func envFloatOrDefault(key string, fallback float64) (float64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid number: %w", key, err)
	}

	return parsed, nil
}

func envBoolOrDefault(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return parsed, nil
}

func loadJWTEncKey() ([]byte, error) {
	if value := strings.TrimSpace(os.Getenv("JWT_ENC_SECRET")); value != "" {
		return deriveJWTEncKey(value)
	}
	if value := strings.TrimSpace(os.Getenv("JWT_ENC_KEY_BASE64")); value != "" {
		return decodeJWTEncKeyBase64("JWT_ENC_KEY_BASE64", value)
	}
	if value := strings.TrimSpace(os.Getenv("JWT_ENC_KEY")); value != "" {
		return decodeJWTEncKeyBase64("JWT_ENC_KEY", value)
	}
	if value := strings.TrimSpace(os.Getenv("JWT_ENC_KEY_HEX")); value != "" {
		if key, err := decodeJWTEncKeyHex(value); err == nil {
			return key, nil
		}
		return decodeJWTEncKeyBase64("JWT_ENC_KEY_HEX", value)
	}

	return nil, errors.New("set JWT_ENC_SECRET, JWT_ENC_KEY_BASE64, JWT_ENC_KEY, or legacy JWT_ENC_KEY_HEX for A256GCM payload decryption")
}

func deriveJWTEncKey(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("JWT_ENC_SECRET must not be empty")
	}

	if key, err := decodeJWTEncKeyHex(trimmed); err == nil {
		return key, nil
	}
	if key, err := decodeJWTEncKeyBase64("JWT_ENC_SECRET", trimmed); err == nil {
		return key, nil
	}

	sum := sha256.Sum256(bestEffortSecretBytes(trimmed))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

func decodeJWTEncKeyHex(value string) ([]byte, error) {
	key, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("hex key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}

	return key, nil
}

func decodeJWTEncKeyBase64(envName, value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("decode %s as base64: %w", envName, err)
		}
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes, got %d bytes", envName, len(key))
	}

	return key, nil
}

func bestEffortSecretBytes(secret string) []byte {
	if decoded, err := hex.DecodeString(secret); err == nil {
		return decoded
	}
	if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil {
		return decoded
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(secret); err == nil {
		return decoded
	}

	return []byte(secret)
}
