package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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
	if value := os.Getenv("RATE_LIMIT_ENABLED"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_ENABLED must be a valid boolean: %v", err)
		}
		config.RateLimit.Enabled = parsed
	}
	if value := os.Getenv("RATE_LIMIT_KEY_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_KEY_TTL must be a valid duration: %v", err)
		}
		config.RateLimit.KeyTTL = parsed
	}
	if value := os.Getenv("TRUSTED_PROXY_CIDRS"); value != "" {
		config.RateLimit.TrustedProxyCIDRs = splitAndTrim(value)
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_ADDR"); value != "" {
		config.RateLimit.Redis.Addr = value
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_PASSWORD"); value != "" {
		config.RateLimit.Redis.Password = value
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_DB"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_REDIS_DB must be a valid integer: %v", err)
		}
		config.RateLimit.Redis.DB = parsed
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_PREFIX"); value != "" {
		config.RateLimit.Redis.KeyPrefix = value
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_DIAL_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_REDIS_DIAL_TIMEOUT must be a valid duration: %v", err)
		}
		config.RateLimit.Redis.DialTimeout = parsed
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_READ_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_REDIS_READ_TIMEOUT must be a valid duration: %v", err)
		}
		config.RateLimit.Redis.ReadTimeout = parsed
	}
	if value := os.Getenv("RATE_LIMIT_REDIS_WRITE_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("RATE_LIMIT_REDIS_WRITE_TIMEOUT must be a valid duration: %v", err)
		}
		config.RateLimit.Redis.WriteTimeout = parsed
	}
	if value := os.Getenv("CACHE_ENABLED"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			log.Fatalf("CACHE_ENABLED must be a valid boolean: %v", err)
		}
		config.Cache.Enabled = parsed
	}
	if value := os.Getenv("CACHE_ADMIN_STATS_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_ADMIN_STATS_TTL must be a valid duration: %v", err)
		}
		config.Cache.AdminStatsTTL = parsed
	}
	if value := os.Getenv("CACHE_UNIVERSITIES_LIST_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_UNIVERSITIES_LIST_TTL must be a valid duration: %v", err)
		}
		config.Cache.UniversitiesListTTL = parsed
	}
	if value := os.Getenv("CACHE_UNIVERSITY_PROFILE_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_UNIVERSITY_PROFILE_TTL must be a valid duration: %v", err)
		}
		config.Cache.UniversityProfileTTL = parsed
	}
	if value := os.Getenv("CACHE_BATCH_STATUS_TTL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_BATCH_STATUS_TTL must be a valid duration: %v", err)
		}
		config.Cache.BatchStatusTTL = parsed
	}
	if value := os.Getenv("CACHE_REDIS_ADDR"); value != "" {
		config.Cache.Redis.Addr = value
	}
	if value := os.Getenv("CACHE_REDIS_PASSWORD"); value != "" {
		config.Cache.Redis.Password = value
	}
	if value := os.Getenv("CACHE_REDIS_DB"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			log.Fatalf("CACHE_REDIS_DB must be a valid integer: %v", err)
		}
		config.Cache.Redis.DB = parsed
	}
	if value := os.Getenv("CACHE_REDIS_PREFIX"); value != "" {
		config.Cache.Redis.KeyPrefix = value
	}
	if value := os.Getenv("CACHE_REDIS_DIAL_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_REDIS_DIAL_TIMEOUT must be a valid duration: %v", err)
		}
		config.Cache.Redis.DialTimeout = parsed
	}
	if value := os.Getenv("CACHE_REDIS_READ_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_REDIS_READ_TIMEOUT must be a valid duration: %v", err)
		}
		config.Cache.Redis.ReadTimeout = parsed
	}
	if value := os.Getenv("CACHE_REDIS_WRITE_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			log.Fatalf("CACHE_REDIS_WRITE_TIMEOUT must be a valid duration: %v", err)
		}
		config.Cache.Redis.WriteTimeout = parsed
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
