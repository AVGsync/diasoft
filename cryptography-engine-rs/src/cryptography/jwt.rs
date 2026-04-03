//! JWT generation for QR codes and share links.
//!
//! Creates two JWT types:
//! - **QR JWT** — RS256, eternal (exp: null), revocation via `diploma_hashes.status`
//! - **Share-link JWT** — HS256, configurable TTL (default 72h)

use jsonwebtoken::{encode, decode, Header, Algorithm, EncodingKey, DecodingKey, Validation};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::config::AppConfig;
use crate::error::{AppError, AppResult};

/// Claims embedded in the QR code JWT (RS256, eternal)
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct QrClaims {
    /// Subject - diploma hash
    pub sub: String,
    /// Diploma hash (repeated for convenience)
    pub diploma_hash: String,
    /// University ID
    pub vuz_id: Uuid,
    /// Diploma number
    pub diploma_number: String,
    /// Student full name
    pub student_name: String,
    /// Specialty
    pub specialty: String,
    /// Graduation year
    pub year: u16,
    /// Salt used for hash computation (allows independent verification)
    pub salt: String,
    /// Issued at timestamp
    pub iat: u64,
}

/// Claims for share-link JWT (HS256, time-limited)
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ShareLinkClaims {
    /// Subject - diploma hash
    pub sub: String,
    /// Diploma hash
    pub diploma_hash: String,
    /// Token type identifier
    #[serde(rename = "type")]
    pub token_type: String,
    /// Issued at timestamp
    pub iat: u64,
    /// Expiration timestamp
    pub exp: u64,
}

/// Creates a QR JWT token with RS256 signature.
///
/// The token is eternal (no expiration) - revocation is handled via
/// the `diploma_hashes.status` column in the database.
///
/// # Arguments
/// * `claims` - QR claims to embed in the token
/// * `config` - Application configuration containing the RSA private key
///
/// # Returns
/// JWT string ready to be embedded in a QR code
pub fn create_qr_jwt(claims: &QrClaims, config: &AppConfig) -> AppResult<String> {
    let key = EncodingKey::from_rsa_pem(config.jwt.qr_rsa_private_key_pem.as_bytes())
        .map_err(|e| AppError::Jwt(e))?;
    
    let token = encode(&Header::new(Algorithm::RS256), claims, &key)?;
    
    Ok(token)
}

/// Builds the verification URL for a QR code JWT.
///
/// # Arguments
/// * `token` - JWT token string
/// * `config` - Application configuration containing the verification base URL
///
/// # Returns
/// Full verification URL: `{verification_base_url}?payload={jwt}`
pub fn build_qr_url(token: &str, config: &AppConfig) -> String {
    format!("{}?payload={}", config.app.verification_base_url, token)
}

/// Creates a share-link JWT token with HS256 signature.
///
/// This token has a limited lifetime (configurable TTL, default 72 hours).
/// The INSERT into `share_links` table is done by the Gateway service;
/// this function only produces the token.
///
/// # Arguments
/// * `diploma_hash` - SHA-256 hash of the diploma
/// * `ttl_hours` - Time-to-live in hours
/// * `config` - Application configuration containing the HMAC secret
///
/// # Returns
/// JWT string for the share link
pub fn create_share_link_jwt(
    diploma_hash: &str,
    ttl_hours: u64,
    config: &AppConfig,
) -> AppResult<String> {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(|e| AppError::Jwt(jsonwebtoken::errors::Error::from(
            jsonwebtoken::errors::ErrorKind::InvalidToken
        )))?
        .as_secs();
    
    let claims = ShareLinkClaims {
        sub: diploma_hash.to_string(),
        diploma_hash: diploma_hash.to_string(),
        token_type: "share_link".to_string(),
        iat: now,
        exp: now + (ttl_hours * 3600),
    };
    
    let key = EncodingKey::from_secret(config.jwt.auth_hmac_secret.as_bytes());
    
    let token = encode(&Header::new(Algorithm::HS256), &claims, &key)?;
    
    Ok(token)
}

/// Verifies and decodes a QR JWT token.
///
/// # Arguments
/// * `token` - JWT token string
/// * `config` - Application configuration containing the RSA public key
///
/// # Returns
/// Decoded QR claims if signature is valid
pub fn verify_qr_jwt(token: &str, public_key_pem: &str) -> AppResult<QrClaims> {
    let key = DecodingKey::from_rsa_pem(public_key_pem.as_bytes())?;
    
    let mut validation = Validation::new(Algorithm::RS256);
    validation.validate_exp = false; // QR tokens don't expire
    
    let data = decode::<QrClaims>(token, &key, &validation)?;
    
    Ok(data.claims)
}

/// Verifies and decodes a share-link JWT token.
///
/// # Arguments
/// * `token` - JWT token string
/// * `config` - Application configuration containing the HMAC secret
///
/// # Returns
/// Decoded share-link claims if signature is valid and not expired
pub fn verify_share_link_jwt(token: &str, config: &AppConfig) -> AppResult<ShareLinkClaims> {
    let key = DecodingKey::from_secret(config.jwt.auth_hmac_secret.as_bytes());
    
    let validation = Validation::new(Algorithm::HS256);
    
    let data = decode::<ShareLinkClaims>(token, &key, &validation)?;
    
    Ok(data.claims)
}

#[cfg(test)]
mod tests {
    use super::*;
    
    fn get_test_config() -> AppConfig {
        // Generate a test RSA key pair
        use rsa::{RsaPrivateKey, pkcs8::EncodePublicKey};
        use rsa::pkcs8::EncodePrivateKey;
        use rand::rngs::OsRng;
        
        let mut rng = OsRng;
        let private_key = RsaPrivateKey::new(&mut rng, 2048).unwrap();
        let private_pem = private_key.to_pkcs8_pem(Default::default()).unwrap();
        
        AppConfig {
            kafka: crate::config::KafkaConfig {
                brokers: "localhost:9092".to_string(),
                group_id: "test".to_string(),
                input_topic: "test.in".to_string(),
                output_topic: "test.out".to_string(),
            },
            database: crate::config::DatabaseConfig {
                url: "postgres://test".to_string(),
            },
            jwt: crate::config::JwtConfig {
                qr_rsa_private_key_pem: private_pem.to_string(),
                auth_hmac_secret: "test-secret-key-32-bytes-long!!".to_string(),
            },
            app: crate::config::AppSettings {
                verification_base_url: "https://test.com/verify".to_string(),
                encryption_key: "0".repeat(64),
            },
        }
    }
    
    #[test]
    fn test_create_share_link_jwt() {
        let config = get_test_config();
        let diploma_hash = "a".repeat(64);
        
        let token = create_share_link_jwt(&diploma_hash, 72, &config).unwrap();
        
        let claims = verify_share_link_jwt(&token, &config).unwrap();
        assert_eq!(claims.diploma_hash, diploma_hash);
        assert_eq!(claims.token_type, "share_link");
    }
    
    #[test]
    fn test_build_qr_url() {
        let config = get_test_config();
        let token = "test-token";
        
        let url = build_qr_url(token, &config);
        assert_eq!(url, "https://test.com/verify?payload=test-token");
    }
}
