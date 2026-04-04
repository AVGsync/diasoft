//! JWT token generation and verification using Ed25519 (EdDSA algorithm).
//!
//! This module provides JWT functionality for diploma QR codes using Ed25519
//! elliptic curve signatures, which are more efficient and secure than RSA.

use jsonwebtoken::{encode, decode, Header, Algorithm, EncodingKey, DecodingKey, Validation};
use serde::{Serialize, Deserialize};
use tracing::debug;
use uuid::Uuid;
use crate::error::{AppError, AppResult};

/// Claims embedded in the QR code JWT for diploma verification.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QrClaims {
    pub sub: Option<String>,
    pub diploma_hash: Option<String>,
    pub vuz_id: Uuid,
    pub diploma_number: String,
    pub student_name: String,
    pub specialty: String,
    pub year: u16,
    pub salt: String,
    pub iat: u64,
    pub exp: Option<u64>,
}

/// Creates a QR code JWT using Ed25519 (EdDSA algorithm).
///
/// # Arguments
/// * `claims` - The diploma claims to embed in the token
/// * `private_key_pem` - Ed25519 private key in PKCS#8 PEM format
///
/// # Returns
/// A signed JWT token using EdDSA algorithm
///
/// # Key Format
/// The private key must be in PKCS#8 PEM format:
/// ```text
/// -----BEGIN PRIVATE KEY-----
/// ...
/// -----END PRIVATE KEY-----
/// ```
pub fn create_qr_jwt(claims: &QrClaims, private_key_pem: &str) -> AppResult<String> {
    // Debug: log first line of PEM to verify format
    debug!(pem_first_line = ?private_key_pem.lines().next(), "Parsing private key for JWT");
    
    // Parse the PEM key using ed25519_dalek (which is more lenient)
    // then convert to DER for jsonwebtoken
    use ed25519_dalek::SigningKey;
    use pkcs8::{DecodePrivateKey, EncodePrivateKey};
    
    let signing_key = SigningKey::from_pkcs8_pem(private_key_pem)
        .map_err(|e| AppError::Jwt(format!("failed to parse Ed25519 private key: {}", e)))?;
    
    // Convert to PKCS#8 DER format for jsonwebtoken
    let der = signing_key.to_pkcs8_der()
        .map_err(|e| AppError::Jwt(format!("failed to convert key to PKCS#8 DER: {}", e)))?;
    
    let encoding_key = EncodingKey::from_ed_der(der.as_bytes());
    
    let token = encode(
        &Header::new(Algorithm::EdDSA),
        claims,
        &encoding_key,
    ).map_err(|e| AppError::Jwt(e.to_string()))?;
    
    debug!(token_len = token.len(), "Successfully created JWT token");
    Ok(token)
}

/// Verifies and decodes a QR code JWT using Ed25519 public key.
///
/// # Arguments
/// * `token` - The JWT token to verify
/// * `public_key_pem` - Ed25519 public key in PEM format
///
/// # Returns
/// The decoded claims if the signature is valid
pub fn verify_qr_jwt(token: &str, public_key_pem: &str) -> AppResult<QrClaims> {
    let mut validation = Validation::new(Algorithm::EdDSA);
    // QR tokens don't require expiration - they can be verified indefinitely
    validation.required_spec_claims.remove("exp");
    
    let decoding_key = DecodingKey::from_ed_pem(public_key_pem.as_bytes())
        .map_err(|e| AppError::Jwt(format!("failed to parse public key: {}", e)))?;
    
    let decoded = decode::<QrClaims>(
        token,
        &decoding_key,
        &validation,
    ).map_err(|e| AppError::Jwt(e.to_string()))?;
    
    Ok(decoded.claims)
}

/// Builds the verification URL for a QR code token.
pub fn build_qr_url(token: &str, config: &crate::config::AppConfig) -> String {
    format!("{}/{}", config.app.verification_base_url, token)
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::SigningKey;
    use rand::rngs::OsRng;

    #[test]
    fn test_eddsa_jwt_roundtrip() {
        // Generate a new Ed25519 keypair
        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();
        
        // Convert to PKCS#8 PEM format for private key
        let private_key_pem = pkcs8::EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        
        // Convert to PEM format for public key
        let public_key_pem = pkcs8::EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");
        
        let claims = QrClaims {
            sub: Some("test-diploma".to_string()),
            diploma_hash: Some("hash123".to_string()),
            vuz_id: Uuid::nil(),
            diploma_number: "TEST-2024-001".to_string(),
            student_name: "Test Student".to_string(),
            specialty: "Computer Science".to_string(),
            year: 2024,
            salt: "randomsalt".to_string(),
            iat: 1000000000,
            exp: None,
        };
        
        // Create JWT
        let token = create_qr_jwt(&claims, &private_key_pem).expect("Failed to create JWT");
        
        // Verify JWT
        let decoded = verify_qr_jwt(&token, &public_key_pem).expect("Failed to verify JWT");
        
        assert_eq!(decoded.diploma_number, claims.diploma_number);
        assert_eq!(decoded.student_name, claims.student_name);
        assert_eq!(decoded.vuz_id, claims.vuz_id);
    }

    #[test]
    fn test_eddsa_jwt_different_keys() {
        let mut csprng = OsRng;
        
        // Generate two different keypairs
        let signing_key1: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key1 = signing_key1.verifying_key();
        
        let signing_key2: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key2 = signing_key2.verifying_key();
        
        let private_key_pem1 = pkcs8::EncodePrivateKey::to_pkcs8_pem(&signing_key1, pkcs8::LineEnding::default())
            .expect("Failed to encode private key 1");
        let public_key_pem1 = pkcs8::EncodePublicKey::to_public_key_pem(&verifying_key1, pkcs8::LineEnding::default())
            .expect("Failed to encode public key 1");
        
        let public_key_pem2 = pkcs8::EncodePublicKey::to_public_key_pem(&verifying_key2, pkcs8::LineEnding::default())
            .expect("Failed to encode public key 2");
        
        let claims = QrClaims {
            sub: Some("test-diploma".to_string()),
            diploma_hash: Some("hash123".to_string()),
            vuz_id: Uuid::nil(),
            diploma_number: "TEST-2024-001".to_string(),
            student_name: "Test Student".to_string(),
            specialty: "Computer Science".to_string(),
            year: 2024,
            salt: "randomsalt".to_string(),
            iat: 1000000000,
            exp: None,
        };
        
        // Create JWT with key 1
        let token = create_qr_jwt(&claims, &private_key_pem1).expect("Failed to create JWT");
        
        // Should verify with key 1
        verify_qr_jwt(&token, &public_key_pem1).expect("Failed to verify with correct key");
        
        // Should fail with key 2
        assert!(verify_qr_jwt(&token, &public_key_pem2).is_err());
    }

    #[test]
    fn test_eddsa_jwt_different_claims() {
        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();
        
        let private_key_pem = pkcs8::EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        let public_key_pem = pkcs8::EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");
        
        let claims1 = QrClaims {
            sub: Some("test-diploma-1".to_string()),
            diploma_hash: Some("hash1".to_string()),
            vuz_id: Uuid::nil(),
            diploma_number: "TEST-2024-001".to_string(),
            student_name: "Student One".to_string(),
            specialty: "Computer Science".to_string(),
            year: 2024,
            salt: "salt1".to_string(),
            iat: 1000000000,
            exp: None,
        };
        
        let claims2 = QrClaims {
            sub: Some("test-diploma-2".to_string()),
            diploma_hash: Some("hash2".to_string()),
            vuz_id: Uuid::nil(),
            diploma_number: "TEST-2024-002".to_string(),
            student_name: "Student Two".to_string(),
            specialty: "Mathematics".to_string(),
            year: 2024,
            salt: "salt2".to_string(),
            iat: 1000000000,
            exp: None,
        };
        
        let token1 = create_qr_jwt(&claims1, &private_key_pem).expect("Failed to create JWT 1");
        let token2 = create_qr_jwt(&claims2, &private_key_pem).expect("Failed to create JWT 2");
        
        // Different claims should produce different tokens
        assert_ne!(token1, token2);
        
        // Verify both tokens decode correctly
        let decoded1 = verify_qr_jwt(&token1, &public_key_pem).expect("Failed to verify JWT 1");
        let decoded2 = verify_qr_jwt(&token2, &public_key_pem).expect("Failed to verify JWT 2");
        
        assert_eq!(decoded1.diploma_number, claims1.diploma_number);
        assert_eq!(decoded2.diploma_number, claims2.diploma_number);
    }

    #[test]
    fn test_eddsa_jwt_tampered_token_fails() {
        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();
        
        let private_key_pem = pkcs8::EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        let public_key_pem = pkcs8::EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");
        
        let claims = QrClaims {
            sub: Some("test-diploma".to_string()),
            diploma_hash: Some("hash123".to_string()),
            vuz_id: Uuid::nil(),
            diploma_number: "TEST-2024-001".to_string(),
            student_name: "Test Student".to_string(),
            specialty: "Computer Science".to_string(),
            year: 2024,
            salt: "randomsalt".to_string(),
            iat: 1000000000,
            exp: None,
        };
        
        let token = create_qr_jwt(&claims, &private_key_pem).expect("Failed to create JWT");
        
        // Tamper with the token by changing a character in the middle
        let mut tampered = token.clone();
        let mid = tampered.len() / 2;
        let chars: Vec<char> = tampered.chars().collect();
        if chars[mid] == 'a' {
            tampered.replace_range(mid..mid+1, "b");
        } else {
            tampered.replace_range(mid..mid+1, "a");
        }
        
        // Tampered token should fail verification
        assert!(verify_qr_jwt(&tampered, &public_key_pem).is_err());
    }
}
