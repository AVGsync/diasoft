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
	if value := os.Getenv("BIND_ADDR"); value != "" {
		config.BindAddr = value
	}
	if value := os.Getenv("LOG_LEVEL"); value != "" {
		config.LogLevel = value
	}
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
	if value := os.Getenv("QR_PAYLOAD_ENCRYPTION_SECRET"); value != "" {
		config.QRPayloadEncryptionSecret = value
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
	if value := os.Getenv("DEMO_UNIVERSITY_NAME"); value != "" {
		config.DemoUniversityName = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_VUZ_CODE"); value != "" {
		config.DemoUniversityVUZCode = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_INN"); value != "" {
		config.DemoUniversityINN = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_OGRN"); value != "" {
		config.DemoUniversityOGRN = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_EMAIL"); value != "" {
		config.DemoUniversityEmail = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_PASSWORD"); value != "" {
		config.DemoUniversityPassword = value
	}
	if value := os.Getenv("DEMO_UNIVERSITY_PRIVATE_KEY_PATH"); value != "" {
		config.DemoUniversityPrivateKey = value
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
