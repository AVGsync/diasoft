package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	RequestTimeout time.Duration
	LogLevel       string
	DBMaxConns     int32
	UseStubData    bool
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    envOrDefault("LOG_LEVEL", "info"),
		DBMaxConns:  int32(envIntOrDefault("DB_MAX_CONNS", 10)),
		UseStubData: envBoolOrDefault("USE_STUB_DATA", false),
	}

	requestTimeout, err := time.ParseDuration(envOrDefault("REQUEST_TIMEOUT", "5s"))
	if err != nil {
		return Config{}, err
	}
	cfg.RequestTimeout = requestTimeout

	if !cfg.UseStubData && cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envBoolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
