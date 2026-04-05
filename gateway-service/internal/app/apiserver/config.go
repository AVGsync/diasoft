package apiserver

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
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
	DemoUniversityName        string             `toml:"demo_university_name"`
	DemoUniversityVUZCode     string             `toml:"demo_university_vuz_code"`
	DemoUniversityINN         string             `toml:"demo_university_inn"`
	DemoUniversityOGRN        string             `toml:"demo_university_ogrn"`
	DemoUniversityEmail       string             `toml:"demo_university_email"`
	DemoUniversityPassword    string             `toml:"demo_university_password"`
	DemoUniversityPrivateKey  string             `toml:"demo_university_private_key_path"`
	RateLimit                 *RateLimitConfig   `toml:"rate_limit"`
	Cache                     *CacheConfig       `toml:"cache"`
	DB                        *postgres.Config   `toml:"db"`
	Kafka                     *kafkainfra.Config `toml:"kafka"`
}

type RateLimitConfig struct {
	Enabled           bool          `toml:"enabled"`
	KeyTTL            time.Duration `toml:"key_ttl"`
	TrustedProxyCIDRs []string      `toml:"trusted_proxy_cidrs"`
	Redis             *RedisConfig  `toml:"redis"`
}

type RedisConfig struct {
	Addr         string        `toml:"addr"`
	Password     string        `toml:"password"`
	DB           int           `toml:"db"`
	KeyPrefix    string        `toml:"key_prefix"`
	DialTimeout  time.Duration `toml:"dial_timeout"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
}

type CacheConfig struct {
	Enabled              bool          `toml:"enabled"`
	AdminStatsTTL        time.Duration `toml:"admin_stats_ttl"`
	UniversitiesListTTL  time.Duration `toml:"universities_list_ttl"`
	UniversityProfileTTL time.Duration `toml:"university_profile_ttl"`
	BatchStatusTTL       time.Duration `toml:"batch_status_ttl"`
	Redis                *RedisConfig  `toml:"redis"`
}

func NewConfig() *Config {
	return &Config{
		BindAddr:                  ":8080",
		LogLevel:                  "info",
		PublicBaseURL:             "http://localhost",
		JWTSecret:                 "",
		ShareJWTSecret:            "",
		SigningKeysMasterKey:      "",
		QRPayloadEncryptionSecret: "",
		AccessTokenTTL:            24 * time.Hour,
		ShareTokenTTL:             72 * time.Hour,
		BootstrapAdminEmail:       "",
		BootstrapAdminPassword:    "",
		DemoUniversityName:        "",
		DemoUniversityVUZCode:     "",
		DemoUniversityINN:         "",
		DemoUniversityOGRN:        "",
		DemoUniversityEmail:       "",
		DemoUniversityPassword:    "",
		DemoUniversityPrivateKey:  "",
		RateLimit: &RateLimitConfig{
			Enabled:           true,
			KeyTTL:            15 * time.Minute,
			TrustedProxyCIDRs: nil,
			Redis: &RedisConfig{
				Addr:         "localhost:6379",
				Password:     "",
				DB:           0,
				KeyPrefix:    "rl:gateway",
				DialTimeout:  3 * time.Second,
				ReadTimeout:  2 * time.Second,
				WriteTimeout: 2 * time.Second,
			},
		},
		Cache: &CacheConfig{
			Enabled:              false,
			AdminStatsTTL:        30 * time.Second,
			UniversitiesListTTL:  1 * time.Minute,
			UniversityProfileTTL: 5 * time.Minute,
			BatchStatusTTL:       15 * time.Second,
			Redis: &RedisConfig{
				Addr:         "localhost:6379",
				Password:     "",
				DB:           0,
				KeyPrefix:    "cache:gateway",
				DialTimeout:  3 * time.Second,
				ReadTimeout:  2 * time.Second,
				WriteTimeout: 2 * time.Second,
			},
		},
		DB:    postgres.NewConfig(),
		Kafka: kafkainfra.NewConfig(),
	}
}

func (c *Config) Validate() error {
	if c.DB == nil {
		return errors.New("db config is required")
	}
	if c.Kafka == nil {
		return errors.New("kafka config is required")
	}
	if c.RateLimit == nil || c.RateLimit.Redis == nil {
		return errors.New("rate limit config is required")
	}
	if c.Cache == nil || c.Cache.Redis == nil {
		return errors.New("cache config is required")
	}
	if strings.TrimSpace(c.BindAddr) == "" {
		return errors.New("BIND_ADDR must not be empty")
	}
	if strings.TrimSpace(c.DB.DatabaseURL) == "" {
		return errors.New("DATABASE_URL or db.database_url is required")
	}
	if strings.TrimSpace(c.JWTSecret) == "" {
		return errors.New("JWT_SECRET is required")
	}
	if strings.TrimSpace(c.ShareJWTSecret) == "" {
		return errors.New("SHARE_JWT_SECRET is required")
	}
	if err := validateBase64Key("SIGNING_KEYS_MASTER_KEY", c.SigningKeysMasterKey); err != nil {
		return err
	}
	if err := validateBase64Key("QR_PAYLOAD_ENCRYPTION_SECRET", c.QRPayloadEncryptionSecret); err != nil {
		return err
	}
	if err := validateCredentialPair("bootstrap admin", c.BootstrapAdminEmail, c.BootstrapAdminPassword); err != nil {
		return err
	}
	if err := validateDemoUniversityConfig(c); err != nil {
		return err
	}
	if len(c.Kafka.Brokers) == 0 {
		return errors.New("KAFKA_BROKERS must contain at least one broker")
	}
	if strings.TrimSpace(c.Kafka.RawTasksTopic) == "" {
		return errors.New("KAFKA_RAW_TOPIC must not be empty")
	}
	if strings.TrimSpace(c.Kafka.ProcessingResultsTopic) == "" {
		return errors.New("KAFKA_RESULTS_TOPIC must not be empty")
	}
	if strings.TrimSpace(c.Kafka.VerificationEventsTopic) == "" {
		return errors.New("KAFKA_VERIFICATION_EVENTS_TOPIC must not be empty")
	}
	if strings.TrimSpace(c.Kafka.ConsumerGroup) == "" {
		return errors.New("KAFKA_CONSUMER_GROUP must not be empty")
	}

	return nil
}

func validateBase64Key(envName, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", envName)
	}

	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return fmt.Errorf("%s must be base64 and decode to 32 bytes", envName)
		}
	}
	if len(decoded) != 32 {
		return fmt.Errorf("%s must decode to exactly 32 bytes, got %d", envName, len(decoded))
	}

	return nil
}

func validateCredentialPair(name, first, second string) error {
	first = strings.TrimSpace(first)
	second = strings.TrimSpace(second)

	switch {
	case first == "" && second == "":
		return nil
	case first == "" || second == "":
		return fmt.Errorf("%s configuration is incomplete", name)
	default:
		return nil
	}
}

func validateDemoUniversityConfig(c *Config) error {
	values := []string{
		c.DemoUniversityName,
		c.DemoUniversityVUZCode,
		c.DemoUniversityINN,
		c.DemoUniversityOGRN,
		c.DemoUniversityEmail,
		c.DemoUniversityPassword,
	}

	filled := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filled++
		}
	}

	if filled == 0 {
		return nil
	}
	if filled != len(values) {
		return errors.New("demo university configuration is incomplete")
	}

	return nil
}
