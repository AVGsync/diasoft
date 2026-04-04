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
		LogLevel:                  "debug",
		PublicBaseURL:             "http://localhost:3000",
		JWTSecret:                 "gateway-access-secret",
		ShareJWTSecret:            "gateway-share-secret",
		SigningKeysMasterKey:      "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		QRPayloadEncryptionSecret: "XpMcu0eI/4SrmsO99dYLUPAwgyarZbrS92RzFmUjTPI=",
		AccessTokenTTL:            24 * time.Hour,
		ShareTokenTTL:             72 * time.Hour,
		BootstrapAdminEmail:       "admin@platform.local",
		BootstrapAdminPassword:    "Admin12345!",
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
