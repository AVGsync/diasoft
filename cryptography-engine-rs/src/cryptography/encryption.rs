//! AES-GCM encryption for student payload data.
//!
//! Encrypts the full student struct so that raw personal data is never
//! stored in plaintext. The resulting ciphertext is stored in
//! `batch_results.encrypted_payload`.

use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use base64::{Engine as _, engine::general_purpose::STANDARD as BASE64};
use rand::RngCore;
use serde::{de::DeserializeOwned, Serialize};

use crate::error::{AppError, AppResult};

/// Encrypts a serializable payload using AES-256-GCM.
///
/// # Arguments
/// * `data` - Data to encrypt (must be JSON-serializable)
/// * `key` - 32-byte encryption key
///
/// # Returns
/// Base64-encoded ciphertext with 12-byte nonce prepended
///
/// # Format
/// `[12-byte nonce][ciphertext]` encoded as base64
pub fn encrypt_payload<T: Serialize>(data: &T, key: &[u8]) -> AppResult<String> {
    // Validate key length
    if key.len() != 32 {
        return Err(AppError::Encryption(
            "encryption key must be 32 bytes".to_string()
        ));
    }
    
    // Serialize data to JSON
    let json = serde_json::to_vec(data)?;
    
    // Create cipher
    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("failed to create cipher: {}", e)))?;
    
    // Generate random nonce
    let mut nonce_bytes = [0u8; 12];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);
    
    // Encrypt
    let ciphertext = cipher
        .encrypt(nonce, json.as_ref())
        .map_err(|e| AppError::Encryption(format!("encryption failed: {}", e)))?;
    
    // Prepend nonce to ciphertext and encode as base64
    let mut result = Vec::with_capacity(12 + ciphertext.len());
    result.extend_from_slice(&nonce_bytes);
    result.extend_from_slice(&ciphertext);
    
    Ok(BASE64.encode(&result))
}

/// Decrypts an AES-256-GCM encrypted payload.
///
/// # Arguments
/// * `ciphertext` - Base64-encoded ciphertext with nonce prepended
/// * `key` - 32-byte encryption key
///
/// # Returns
/// Deserialized data of type T
pub fn decrypt_payload<T: DeserializeOwned>(ciphertext: &str, key: &[u8]) -> AppResult<T> {
    // Validate key length
    if key.len() != 32 {
        return Err(AppError::Encryption(
            "encryption key must be 32 bytes".to_string()
        ));
    }
    
    // Decode base64
    let encrypted = BASE64.decode(ciphertext)
        .map_err(|e| AppError::Encryption(format!("invalid base64: {}", e)))?;
    
    // Ensure minimum length (12-byte nonce + at least 16-byte tag)
    if encrypted.len() < 28 {
        return Err(AppError::Encryption("ciphertext too short".to_string()));
    }
    
    // Extract nonce and ciphertext
    let nonce = Nonce::from_slice(&encrypted[..12]);
    let ct = &encrypted[12..];
    
    // Create cipher
    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("failed to create cipher: {}", e)))?;
    
    // Decrypt
    let plaintext = cipher
        .decrypt(nonce, ct)
        .map_err(|e| AppError::Encryption(format!("decryption failed: {}", e)))?;
    
    // Deserialize JSON
    let data = serde_json::from_slice(&plaintext)?;
    
    Ok(data)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::{Deserialize, Serialize};
    
    #[derive(Debug, Serialize, Deserialize, PartialEq)]
    struct TestData {
        name: String,
        value: i32,
    }
    
    #[test]
    fn test_encrypt_decrypt_roundtrip() {
        let key = [0u8; 32];
        let data = TestData {
            name: "test".to_string(),
            value: 42,
        };
        
        let encrypted = encrypt_payload(&data, &key).unwrap();
        let decrypted: TestData = decrypt_payload(&encrypted, &key).unwrap();
        
        assert_eq!(data, decrypted);
    }
    
    #[test]
    fn test_encrypt_produces_different_ciphertext() {
        let key = [0u8; 32];
        let data = TestData {
            name: "test".to_string(),
            value: 42,
        };
        
        let encrypted1 = encrypt_payload(&data, &key).unwrap();
        let encrypted2 = encrypt_payload(&data, &key).unwrap();
        
        // Different due to random nonce
        assert_ne!(encrypted1, encrypted2);
    }
    
    #[test]
    fn test_decrypt_wrong_key() {
        let key1 = [0u8; 32];
        let key2 = [1u8; 32];
        let data = TestData {
            name: "test".to_string(),
            value: 42,
        };
        
        let encrypted = encrypt_payload(&data, &key1).unwrap();
        let result: Result<TestData, _> = decrypt_payload(&encrypted, &key2);
        
        assert!(result.is_err());
    }
    
    #[test]
    fn test_invalid_key_length() {
        let key = [0u8; 16]; // Wrong length
        let data = TestData {
            name: "test".to_string(),
            value: 42,
        };
        
        let result = encrypt_payload(&data, &key);
        assert!(result.is_err());
    }
}
