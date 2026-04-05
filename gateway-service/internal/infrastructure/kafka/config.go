package kafka

type Config struct {
	Brokers                 []string `toml:"brokers"`
	RawTasksTopic           string   `toml:"raw_tasks_topic"`
	ProcessingResultsTopic  string   `toml:"processing_results_topic"`
	ConsumerGroup           string   `toml:"consumer_group"`
	VerificationEventsTopic string   `toml:"verification_events_topic"`
	VerificationEventsGroup string   `toml:"verification_events_group"`
	ClientID                string   `toml:"client_id"`
}

func NewConfig() *Config {
	return &Config{
		Brokers:                 []string{"localhost:9092"},
		RawTasksTopic:           "diplomas.raw_tasks",
		ProcessingResultsTopic:  "diplomas.processing_results",
		ConsumerGroup:           "gateway-service",
		VerificationEventsTopic: "verification.events",
		VerificationEventsGroup: "gateway-service-verifications",
		ClientID:                "gateway-service",
	}
}
