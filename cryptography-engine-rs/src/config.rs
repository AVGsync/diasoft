use std::env;

use config::ConfigError;
use serde::Deserialize;

#[derive(Deserialize, Clone)]
pub struct AppConfig {
    pub kafka:    KafkaConfig,
    pub database: DatabaseConfig,
    pub jwt:      JwtConfig,
    pub app:      AppSettings,
}

#[derive(Deserialize, Clone)]
pub struct KafkaConfig {
    pub brokers:      String,
    pub group_id:     String,
    pub input_topic:  String,
    pub output_topic: String,
}

#[derive(Deserialize, Clone)]
pub struct DatabaseConfig {
    pub url: String,
}

#[derive(Deserialize, Clone)]
pub struct JwtConfig {
    pub auth_hmac_secret: String,
    pub payload_secret:   String,
}

#[derive(Deserialize, Clone)]
pub struct AppSettings {
    pub verification_base_url: String,
    pub encryption_key:        String,
}

impl AppConfig {
    pub fn load() -> Result<Self, ConfigError> {
        dotenvy::dotenv().ok();

        let mut cfg: AppConfig = config::Config::builder()
            .set_default("kafka.brokers", "")?
            .set_default("kafka.group_id", "crypto-engine")?
            .set_default("kafka.input_topic", "diplomas.raw_tasks")?
            .set_default("kafka.output_topic", "diplomas.processing_results")?
            .set_default("database.url", "")?
            .set_default("jwt.auth_hmac_secret", "")?
            .set_default("jwt.payload_secret", "")?
            .set_default("app.verification_base_url", "")?
            .set_default("app.encryption_key", "")?
            .add_source(config::File::with_name("config/default").required(false))
            .add_source(config::Environment::with_prefix("APP").separator("__"))
            .build()?
            .try_deserialize()?;

        cfg.apply_env_fallbacks()?;
        cfg.validate()?;

        Ok(cfg)
    }

    fn apply_env_fallbacks(&mut self) -> Result<(), ConfigError> {
        if self.database.url.trim().is_empty() {
            self.database.url = resolve_database_url()?;
        }
        if self.jwt.auth_hmac_secret.trim().is_empty() {
            self.jwt.auth_hmac_secret = first_non_empty_env(&["SHARE_JWT_SECRET"]).unwrap_or_default();
        }
        if self.jwt.payload_secret.trim().is_empty() {
            self.jwt.payload_secret = first_non_empty_env(&["QR_PAYLOAD_ENCRYPTION_SECRET"]).unwrap_or_default();
        }
        if self.app.encryption_key.trim().is_empty() {
            self.app.encryption_key = first_non_empty_env(&["SIGNING_KEYS_MASTER_KEY"]).unwrap_or_default();
        }
        if self.app.verification_base_url.trim().is_empty() {
            if let Some(public_base_url) = first_non_empty_env(&["PUBLIC_BASE_URL"]) {
                let normalized = public_base_url.trim_end_matches('/');
                self.app.verification_base_url = format!("{normalized}/verify?payload=");
            }
        }
        if self.kafka.brokers.trim().is_empty() {
            self.kafka.brokers = first_non_empty_env(&["KAFKA_BROKERS"]).unwrap_or_default();
        }

        Ok(())
    }

    fn validate(&self) -> Result<(), ConfigError> {
        if self.database.url.trim().is_empty() {
            return Err(ConfigError::Message(
                "DATABASE_URL or APP__DATABASE__URL is required".into(),
            ));
        }
        if self.kafka.brokers.trim().is_empty() {
            return Err(ConfigError::Message(
                "KAFKA_BROKERS or APP__KAFKA__BROKERS is required".into(),
            ));
        }
        if self.kafka.group_id.trim().is_empty() {
            return Err(ConfigError::Message(
                "APP__KAFKA__GROUP_ID must not be empty".into(),
            ));
        }
        if self.kafka.input_topic.trim().is_empty() {
            return Err(ConfigError::Message(
                "APP__KAFKA__INPUT_TOPIC must not be empty".into(),
            ));
        }
        if self.kafka.output_topic.trim().is_empty() {
            return Err(ConfigError::Message(
                "APP__KAFKA__OUTPUT_TOPIC must not be empty".into(),
            ));
        }
        if self.jwt.auth_hmac_secret.trim().is_empty() {
            return Err(ConfigError::Message(
                "SHARE_JWT_SECRET or APP__JWT__AUTH_HMAC_SECRET is required".into(),
            ));
        }
        validate_base64_key(
            "QR_PAYLOAD_ENCRYPTION_SECRET or APP__JWT__PAYLOAD_SECRET",
            &self.jwt.payload_secret,
        )?;
        validate_base64_key(
            "SIGNING_KEYS_MASTER_KEY or APP__APP__ENCRYPTION_KEY",
            &self.app.encryption_key,
        )?;
        if self.app.verification_base_url.trim().is_empty() {
            return Err(ConfigError::Message(
                "PUBLIC_BASE_URL or APP__APP__VERIFICATION_BASE_URL is required".into(),
            ));
        }

        Ok(())
    }
}

fn resolve_database_url() -> Result<String, ConfigError> {
    let host = first_non_empty_env(&["POSTGRES_HOST"]).unwrap_or_default();
    let database = first_non_empty_env(&["POSTGRES_DB"]).unwrap_or_default();
    let user = first_non_empty_env(&["POSTGRES_USER"]).unwrap_or_default();

    if !host.is_empty() && !database.is_empty() && !user.is_empty() {
        let port = first_non_empty_env(&["POSTGRES_PORT"]).unwrap_or_else(|| "5432".to_string());
        let password = env::var("POSTGRES_PASSWORD").unwrap_or_default();
        let sslmode = first_non_empty_env(&["POSTGRES_SSLMODE"]).unwrap_or_else(|| "disable".to_string());

        return Ok(format!(
            "postgres://{user}:{password}@{host}:{port}/{database}?sslmode={sslmode}"
        ));
    }

    if let Some(database_url) = first_non_empty_env(&["DATABASE_URL"]) {
        return Ok(database_url);
    }

    Ok(String::new())
}

fn first_non_empty_env(keys: &[&str]) -> Option<String> {
    keys.iter().find_map(|key| {
        env::var(key).ok().and_then(|value| {
            let trimmed = value.trim();
            if trimmed.is_empty() {
                None
            } else {
                Some(trimmed.to_string())
            }
        })
    })
}

fn validate_base64_key(name: &str, value: &str) -> Result<(), ConfigError> {
    use base64::{engine::general_purpose::STANDARD as BASE64, engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};

    let trimmed = value.trim();
    if trimmed.is_empty() {
        return Err(ConfigError::Message(format!("{name} is required")));
    }

    let decoded = BASE64
        .decode(trimmed)
        .or_else(|_| URL_SAFE_NO_PAD.decode(trimmed))
        .map_err(|_| ConfigError::Message(format!("{name} must be base64 and decode to 32 bytes")))?;

    if decoded.len() != 32 {
        return Err(ConfigError::Message(format!(
            "{name} must decode to exactly 32 bytes, got {}",
            decoded.len()
        )));
    }

    Ok(())
}
