package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/diasoft/gateway-service/internal/app/apiserver"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config-path", "configs/apiserver.toml", "path to config file")
}

func main() {
	flag.Parse()

	config := apiserver.NewConfig()
	if _, err := toml.DecodeFile(configPath, config); err != nil {
		log.Fatal(err)
	}

	applyEnvOverrides(config)

	server := apiserver.New(config)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}

func applyEnvOverrides(config *apiserver.Config) {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		config.DB.DatabaseURL = value
	}
	if value := os.Getenv("JWT_SECRET"); value != "" {
		config.JWTSecret = value
	}
	if value := os.Getenv("SHARE_JWT_SECRET"); value != "" {
		config.ShareJWTSecret = value
	}
	if value := os.Getenv("SIGNING_KEYS_MASTER_KEY"); value != "" {
		config.SigningKeysMasterKey = value
	}
	if value := os.Getenv("PUBLIC_BASE_URL"); value != "" {
		config.PublicBaseURL = value
	}
	if value := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"); value != "" {
		config.BootstrapAdminEmail = value
	}
	if value := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"); value != "" {
		config.BootstrapAdminPassword = value
	}
	if value := os.Getenv("KAFKA_BROKERS"); value != "" {
		config.Kafka.Brokers = splitAndTrim(value)
	}
	if value := os.Getenv("KAFKA_RAW_TOPIC"); value != "" {
		config.Kafka.RawTasksTopic = value
	}
	if value := os.Getenv("KAFKA_RESULTS_TOPIC"); value != "" {
		config.Kafka.ProcessingResultsTopic = value
	}
	if value := os.Getenv("KAFKA_CONSUMER_GROUP"); value != "" {
		config.Kafka.ConsumerGroup = value
	}
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
