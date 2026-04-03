//! Central error enum used across every module.
//!
//! This module imports nothing internal — it is a safe dependency leaf.
//! All variants can be converted from underlying library errors using `#[from]`.

use thiserror::Error;

/// Application-wide error type
#[derive(Debug, Error)]
pub enum AppError {
    /// SHA-256 or salt generation failures
    #[error("hashing error: {0}")]
    Hashing(String),
    
    /// Ed25519 key parsing or signature errors
    #[error("signing error: {0}")]
    Signing(String),
    
    /// AES-GCM cipher failures
    #[error("encryption error: {0}")]
    Encryption(String),
    
    /// JWT encode/decode errors
    #[error("JWT error: {0}")]
    Jwt(#[from] jsonwebtoken::errors::Error),
    
    /// PostgreSQL database errors
    #[error("database error: {0}")]
    Db(#[from] sqlx::Error),
    
    /// Kafka producer/consumer errors
    #[error("kafka error: {0}")]
    Kafka(#[from] rdkafka::error::KafkaError),
    
    /// JSON (de)serialization errors
    #[error("serialization error: {0}")]
    Serde(#[from] serde_json::Error),
    
    /// Configuration loading errors
    #[error("configuration error: {0}")]
    Config(#[from] config::ConfigError),
    
    /// IO errors
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

/// Convenience type alias for Result with AppError
pub type AppResult<T> = Result<T, AppError>;
