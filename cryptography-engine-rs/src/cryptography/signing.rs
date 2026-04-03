//! Ed25519 digital signature operations.
//!
//! Each university registers its key pair during onboarding; the private key
//! is stored in `universities` table. It is fetched from the DB at processing
//! time — never kept in memory between requests.

use base64::{Engine as _, engine::general_purpose::STANDARD as BASE64};
use ed25519_dalek::{Signature, SigningKey, VerifyingKey};
use pkcs8::DecodePrivateKey;

use crate::error::{AppError, AppResult};

/// Signs a diploma hash using the university's Ed25519 private key.
///
/// # Arguments
/// * `hash` - 64-character hex-encoded SHA-256 hash
/// * `private_key_pem` - Ed25519 private key in PEM format (PKCS#8)
///
/// # Returns
/// Base64-encoded Ed25519 signature
pub fn sign_hash(hash: &str, private_key_pem: &str) -> AppResult<String> {
    // Parse the private key from PEM format
    let signing_key = parse_private_key(private_key_pem)?;
    
    // Decode hex hash to bytes
    let hash_bytes = hex::decode(hash)
        .map_err(|e| AppError::Signing(format!("invalid hex hash: {}", e)))?;
    
    // Sign the hash
    let signature = signing_key.sign(&hash_bytes);
    
    // Encode signature as base64
    Ok(BASE64.encode(signature.to_bytes()))
}

/// Verifies an Ed25519 signature against a diploma hash.
///
/// Used by the Verifier service; lives here for library reuse.
///
/// # Arguments
/// * `hash` - 64-character hex-encoded SHA-256 hash
/// * `signature` - Base64-encoded Ed25519 signature
/// * `public_key_pem` - Ed25519 public key in PEM format
///
/// # Returns
/// `true` if signature is valid, `false` otherwise
pub fn verify_signature(hash: &str, signature: &str, public_key_pem: &str) -> AppResult<bool> {
    // Parse the public key from PEM format
    let verifying_key = parse_public_key(public_key_pem)?;
    
    // Decode hex hash to bytes
    let hash_bytes = hex::decode(hash)
        .map_err(|e| AppError::Signing(format!("invalid hex hash: {}", e)))?;
    
    // Decode base64 signature
    let sig_bytes = BASE64.decode(signature)
        .map_err(|e| AppError::Signing(format!("invalid base64 signature: {}", e)))?;
    
    let signature = Signature::from_slice(&sig_bytes)
        .map_err(|e| AppError::Signing(format!("invalid signature bytes: {}", e)))?;
    
    // Verify the signature
    use ed25519_dalek::Verifier;
    Ok(verifying_key.verify(&hash_bytes, &signature).is_ok())
}

/// Parse an Ed25519 private key from PEM format.
fn parse_private_key(pem: &str) -> AppResult<SigningKey> {
    // Try to parse as PKCS#8 PEM
    let signing_key = SigningKey::from_pkcs8_pem(pem)
        .map_err(|e| AppError::Signing(format!("failed to parse private key: {}", e)))?;
    
    Ok(signing_key)
}

/// Parse an Ed25519 public key from PEM format.
fn parse_public_key(pem: &str) -> AppResult<VerifyingKey> {
    // Extract the verifying key from the signing key if it's a private key PEM
    // or parse directly as public key
    let verifying_key = if pem.contains("PRIVATE KEY") {
        let signing_key = parse_private_key(pem)?;
        signing_key.verifying_key()
    } else {
        // Try parsing as public key PEM
        VerifyingKey::from_public_key_pem(pem)
            .map_err(|e| AppError::Signing(format!("failed to parse public key: {}", e)))?
    };
    
    Ok(verifying_key)
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::SigningKey;
    use rand::rngs::OsRng;
    
    fn generate_test_keypair() -> (String, String) {
        let signing_key = SigningKey::generate(&mut OsRng);
        let verifying_key = signing_key.verifying_key();
        
        let private_pem = signing_key.to_pkcs8_pem(Default::default())
            .unwrap()
            .to_string();
        let public_pem = verifying_key.to_public_key_pem(Default::default())
            .unwrap();
        
        (private_pem, public_pem)
    }
    
    #[test]
    fn test_sign_and_verify() {
        let (private_pem, public_pem) = generate_test_keypair();
        let hash = "a".repeat(64); // 64 hex chars = 32 bytes
        
        let signature = sign_hash(&hash, &private_pem).unwrap();
        let is_valid = verify_signature(&hash, &signature, &public_pem).unwrap();
        
        assert!(is_valid);
    }
    
    #[test]
    fn test_verify_wrong_signature() {
        let (private_pem, public_pem) = generate_test_keypair();
        let hash = "a".repeat(64);
        
        let signature = sign_hash(&hash, &private_pem).unwrap();
        let wrong_hash = "b".repeat(64);
        
        let is_valid = verify_signature(&wrong_hash, &signature, &public_pem).unwrap();
        assert!(!is_valid);
    }
}
