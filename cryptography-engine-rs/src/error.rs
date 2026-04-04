use thiserror::Error;

#[derive(Debug, Error)]
pub enum AppError {
    #[error("hashing error: {0}")]
    Hashing(String),
    
    #[error("signing error: {0}")]
    Signing(String),
    
    #[error("encryption error: {0}")]
    Encryption(String),
    
    #[error("JWT error: {0}")]
    Jwt(String),
    
    #[error("database error: {0}")]
    Db(#[from] sqlx::Error),
    
    #[error("kafka error: {0}")]
    Kafka(#[from] rdkafka::error::KafkaError),
    
    #[error("serialization error: {0}")]
    Serde(#[from] serde_json::Error),
    
    #[error("configuration error: {0}")]
    Config(#[from] config::ConfigError),
    
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

pub type AppResult<T> = Result<T, AppError>;
