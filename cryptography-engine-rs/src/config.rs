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
    pub brokers:           String,
    pub group_id:          String,
    pub input_topic:       String,
    pub output_topic:      String,
}

#[derive(Deserialize, Clone)]
pub struct DatabaseConfig {
    pub url: String,
}

#[derive(Deserialize, Clone)]
pub struct JwtConfig {
    pub auth_hmac_secret: String,
    pub payload_secret: String,
}

#[derive(Deserialize, Clone)]
pub struct AppSettings {
    pub verification_base_url: String,
    pub encryption_key: Option<String>,
}

impl AppConfig {
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
