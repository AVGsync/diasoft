//! Application configuration loaded from environment variables and config files.
//!
//! Owns the entire application configuration as a strongly-typed struct.
//! Loaded once at startup; passed by reference (or cheap `Clone`) everywhere.

use serde::Deserialize;

/// Top-level application configuration
#[derive(Deserialize, Clone)]
pub struct AppConfig {
    pub kafka:    KafkaConfig,
    pub database: DatabaseConfig,
    pub jwt:      JwtConfig,
    pub app:      AppSettings,
}

/// Kafka connection and topic configuration
#[derive(Deserialize, Clone)]
pub struct KafkaConfig {
    /// Kafka broker addresses (comma-separated)
    pub brokers:           String,
    /// Consumer group ID
    pub group_id:          String,
    /// Input topic for raw diploma tasks (diplomas.raw_tasks)
    pub input_topic:       String,
    /// Output topic for processing results (diplomas.processing_results)
    pub output_topic:      String,
}

/// PostgreSQL database configuration
#[derive(Deserialize, Clone)]
pub struct DatabaseConfig {
    /// PostgreSQL connection URL
    pub url: String,
}

/// JWT configuration for QR tokens and share-link tokens
#[derive(Deserialize, Clone)]
pub struct JwtConfig {
    /// RS256 RSA private key in PEM format for signing QR JWTs (eternal tokens)
    pub qr_rsa_private_key_pem: String,
    /// HMAC secret for HS256 share-link JWTs
    pub auth_hmac_secret: String,
}

/// Application-level settings
#[derive(Deserialize, Clone)]
pub struct AppSettings {
    /// Base URL for verification endpoint (e.g. https://platform.ru/api/v1/verify)
    pub verification_base_url: String,
    /// AES-256 encryption key (32 bytes, hex-encoded)
    pub encryption_key: String,
}

impl AppConfig {
    /// Load configuration from environment variables and config files.
    ///
    /// Uses the `config` crate with:
    /// - `config/default.{toml,json,yaml}` as base configuration
    /// - `APP__<section>__<key>` environment variables as overrides
    pub fn load() -> Result<Self, config::ConfigError> {
        dotenvy::dotenv().ok();
        
        let cfg = config::Config::builder()
            .add_source(config::File::with_name("config/default").required(false))
            .add_source(config::Environment::with_prefix("APP").separator("__"))
            .build()?
            .try_deserialize()?;
        
        Ok(cfg)
    }
}
