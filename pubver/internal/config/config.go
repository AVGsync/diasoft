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
	Cache          CacheConfig
	Analytics      AnalyticsConfig
}

type RateLimitConfig struct {
	Enabled           bool
	VerifyRPS         float64
	VerifyBurst       int
	SearchRPS         float64
	SearchBurst       int
	KeyTTL            time.Duration
	TrustedProxyCIDRs []string
	Redis             RedisConfig
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	KeyPrefix    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type CacheConfig struct {
	Enabled                bool
	UniversityKeyTTL       time.Duration
	DiplomaRecordByHashTTL time.Duration
	DiplomaSearchResultTTL time.Duration
	Redis                  RedisConfig
}

type AnalyticsConfig struct {
	Enabled      bool
	KafkaBrokers []string
	KafkaTopic   string
	ClientID     string
	WriteTimeout time.Duration
	QueueSize    int
	GeoIPDBPath  string
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
	verifyBurst, err := envIntOrDefault("RATE_LIMIT_VERIFY_BURST", 5)
	if err != nil {
		return Config{}, err
	}
	searchBurst, err := envIntOrDefault("RATE_LIMIT_SEARCH_BURST", 3)
	if err != nil {
		return Config{}, err
	}
	verifyRPS, err := envFloatOrDefault("RATE_LIMIT_VERIFY_RPS", 0.25)
	if err != nil {
		return Config{}, err
	}
	searchRPS, err := envFloatOrDefault("RATE_LIMIT_SEARCH_RPS", 0.1)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := envIntOrDefault("RATE_LIMIT_REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	cacheEnabled, err := envBoolOrDefault("CACHE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	analyticsEnabled, err := envBoolOrDefault("ANALYTICS_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	cacheRedisDB, err := envIntOrDefault("CACHE_REDIS_DB", redisDB)
	if err != nil {
		return Config{}, err
	}
	analyticsQueueSize, err := envIntOrDefault("ANALYTICS_QUEUE_SIZE", 1024)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    envOrDefault("LOG_LEVEL", "info"),
		DBMaxConns:  int32(dbMaxConns),
		RateLimit: RateLimitConfig{
			Enabled:     rateLimitEnabled,
			VerifyRPS:   verifyRPS,
			VerifyBurst: verifyBurst,
			SearchRPS:   searchRPS,
			SearchBurst: searchBurst,
			Redis: RedisConfig{
				Addr:         strings.TrimSpace(os.Getenv("RATE_LIMIT_REDIS_ADDR")),
				Password:     envOrDefault("RATE_LIMIT_REDIS_PASSWORD", os.Getenv("REDIS_PASSWORD")),
				DB:           redisDB,
				KeyPrefix:    strings.TrimSpace(envOrDefault("RATE_LIMIT_REDIS_PREFIX", "rl:pubver")),
				DialTimeout:  3 * time.Second,
				ReadTimeout:  2 * time.Second,
				WriteTimeout: 2 * time.Second,
			},
			TrustedProxyCIDRs: splitAndTrim(os.Getenv("TRUSTED_PROXY_CIDRS")),
		},
		Cache: CacheConfig{
			Enabled: cacheEnabled,
			Redis: RedisConfig{
				Addr:         strings.TrimSpace(envOrDefault("CACHE_REDIS_ADDR", os.Getenv("RATE_LIMIT_REDIS_ADDR"))),
				Password:     envOrDefault("CACHE_REDIS_PASSWORD", envOrDefault("RATE_LIMIT_REDIS_PASSWORD", os.Getenv("REDIS_PASSWORD"))),
				DB:           cacheRedisDB,
				KeyPrefix:    strings.TrimSpace(envOrDefault("CACHE_REDIS_PREFIX", "cache:pubver")),
				DialTimeout:  3 * time.Second,
				ReadTimeout:  2 * time.Second,
				WriteTimeout: 2 * time.Second,
			},
		},
		Analytics: AnalyticsConfig{
			Enabled:      analyticsEnabled,
			KafkaBrokers: splitAndTrim(envOrDefault("ANALYTICS_KAFKA_BROKERS", os.Getenv("KAFKA_BROKERS"))),
			KafkaTopic:   strings.TrimSpace(envOrDefault("ANALYTICS_KAFKA_TOPIC", "verification.events")),
			ClientID:     strings.TrimSpace(envOrDefault("ANALYTICS_KAFKA_CLIENT_ID", "pubver-analytics")),
			WriteTimeout: 3 * time.Second,
			QueueSize:    analyticsQueueSize,
			GeoIPDBPath:  strings.TrimSpace(os.Getenv("ANALYTICS_GEOIP_DB_PATH")),
		},
	}

	requestTimeout, err := time.ParseDuration(envOrDefault("REQUEST_TIMEOUT", "5s"))
	if err != nil {
		return Config{}, err
	}
	cfg.RequestTimeout = requestTimeout

	rateLimitKeyTTL, err := time.ParseDuration(envOrDefault("RATE_LIMIT_KEY_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_KEY_TTL must be a valid duration: %w", err)
	}
	cfg.RateLimit.KeyTTL = rateLimitKeyTTL

	rateLimitDialTimeout, err := time.ParseDuration(envOrDefault("RATE_LIMIT_REDIS_DIAL_TIMEOUT", "3s"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_REDIS_DIAL_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.RateLimit.Redis.DialTimeout = rateLimitDialTimeout

	rateLimitReadTimeout, err := time.ParseDuration(envOrDefault("RATE_LIMIT_REDIS_READ_TIMEOUT", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_REDIS_READ_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.RateLimit.Redis.ReadTimeout = rateLimitReadTimeout

	rateLimitWriteTimeout, err := time.ParseDuration(envOrDefault("RATE_LIMIT_REDIS_WRITE_TIMEOUT", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("RATE_LIMIT_REDIS_WRITE_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.RateLimit.Redis.WriteTimeout = rateLimitWriteTimeout

	if cfg.RateLimit.Enabled {
		switch {
		case cfg.RateLimit.VerifyRPS <= 0:
			return Config{}, errors.New("RATE_LIMIT_VERIFY_RPS must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.VerifyBurst <= 0:
			return Config{}, errors.New("RATE_LIMIT_VERIFY_BURST must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.SearchRPS <= 0:
			return Config{}, errors.New("RATE_LIMIT_SEARCH_RPS must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.SearchBurst <= 0:
			return Config{}, errors.New("RATE_LIMIT_SEARCH_BURST must be greater than zero when rate limiting is enabled")
		case cfg.RateLimit.KeyTTL <= 0:
			return Config{}, errors.New("RATE_LIMIT_KEY_TTL must be greater than zero when rate limiting is enabled")
		case strings.TrimSpace(cfg.RateLimit.Redis.Addr) == "":
			return Config{}, errors.New("RATE_LIMIT_REDIS_ADDR must be set when rate limiting is enabled")
		case strings.TrimSpace(cfg.RateLimit.Redis.KeyPrefix) == "":
			return Config{}, errors.New("RATE_LIMIT_REDIS_PREFIX must not be empty when rate limiting is enabled")
		}
	}

	universityKeyTTL, err := time.ParseDuration(envOrDefault("CACHE_UNIVERSITY_KEY_TTL", "30m"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_UNIVERSITY_KEY_TTL must be a valid duration: %w", err)
	}
	cfg.Cache.UniversityKeyTTL = universityKeyTTL

	diplomaByHashTTL, err := time.ParseDuration(envOrDefault("CACHE_DIPLOMA_BY_HASH_TTL", "1m"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_DIPLOMA_BY_HASH_TTL must be a valid duration: %w", err)
	}
	cfg.Cache.DiplomaRecordByHashTTL = diplomaByHashTTL

	diplomaSearchTTL, err := time.ParseDuration(envOrDefault("CACHE_DIPLOMA_SEARCH_TTL", "1m"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_DIPLOMA_SEARCH_TTL must be a valid duration: %w", err)
	}
	cfg.Cache.DiplomaSearchResultTTL = diplomaSearchTTL

	cacheDialTimeout, err := time.ParseDuration(envOrDefault("CACHE_REDIS_DIAL_TIMEOUT", "3s"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_REDIS_DIAL_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.Cache.Redis.DialTimeout = cacheDialTimeout

	cacheReadTimeout, err := time.ParseDuration(envOrDefault("CACHE_REDIS_READ_TIMEOUT", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_REDIS_READ_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.Cache.Redis.ReadTimeout = cacheReadTimeout

	cacheWriteTimeout, err := time.ParseDuration(envOrDefault("CACHE_REDIS_WRITE_TIMEOUT", "2s"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_REDIS_WRITE_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.Cache.Redis.WriteTimeout = cacheWriteTimeout

	if cfg.Cache.Enabled {
		switch {
		case strings.TrimSpace(cfg.Cache.Redis.Addr) == "":
			return Config{}, errors.New("CACHE_REDIS_ADDR must be set when cache is enabled")
		case strings.TrimSpace(cfg.Cache.Redis.KeyPrefix) == "":
			return Config{}, errors.New("CACHE_REDIS_PREFIX must not be empty when cache is enabled")
		case cfg.Cache.UniversityKeyTTL <= 0:
			return Config{}, errors.New("CACHE_UNIVERSITY_KEY_TTL must be greater than zero when cache is enabled")
		case cfg.Cache.DiplomaRecordByHashTTL <= 0:
			return Config{}, errors.New("CACHE_DIPLOMA_BY_HASH_TTL must be greater than zero when cache is enabled")
		case cfg.Cache.DiplomaSearchResultTTL <= 0:
			return Config{}, errors.New("CACHE_DIPLOMA_SEARCH_TTL must be greater than zero when cache is enabled")
		}
	}

	analyticsWriteTimeout, err := time.ParseDuration(envOrDefault("ANALYTICS_KAFKA_WRITE_TIMEOUT", "3s"))
	if err != nil {
		return Config{}, fmt.Errorf("ANALYTICS_KAFKA_WRITE_TIMEOUT must be a valid duration: %w", err)
	}
	cfg.Analytics.WriteTimeout = analyticsWriteTimeout

	if cfg.Analytics.Enabled {
		switch {
		case len(cfg.Analytics.KafkaBrokers) == 0:
			return Config{}, errors.New("ANALYTICS_KAFKA_BROKERS must be set when analytics is enabled")
		case strings.TrimSpace(cfg.Analytics.KafkaTopic) == "":
			return Config{}, errors.New("ANALYTICS_KAFKA_TOPIC must not be empty when analytics is enabled")
		case strings.TrimSpace(cfg.Analytics.ClientID) == "":
			return Config{}, errors.New("ANALYTICS_KAFKA_CLIENT_ID must not be empty when analytics is enabled")
		case cfg.Analytics.WriteTimeout <= 0:
			return Config{}, errors.New("ANALYTICS_KAFKA_WRITE_TIMEOUT must be greater than zero when analytics is enabled")
		case cfg.Analytics.QueueSize <= 0:
			return Config{}, errors.New("ANALYTICS_QUEUE_SIZE must be greater than zero when analytics is enabled")
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
	if value := strings.TrimSpace(os.Getenv("QR_PAYLOAD_ENCRYPTION_SECRET")); value != "" {
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

	return nil, errors.New("set JWT_ENC_SECRET, QR_PAYLOAD_ENCRYPTION_SECRET, JWT_ENC_KEY_BASE64, JWT_ENC_KEY, or legacy JWT_ENC_KEY_HEX for A256GCM payload decryption")
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

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
