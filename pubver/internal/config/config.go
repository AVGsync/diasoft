package config

import (
	"errors"
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

	if !cfg.UseStubData {
		cfg.DatabaseURL, err = loadDatabaseURL()
		if err != nil {
			return Config{}, err
		}
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
