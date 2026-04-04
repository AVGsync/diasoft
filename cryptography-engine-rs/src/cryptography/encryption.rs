use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use base64::{Engine as _, engine::general_purpose::STANDARD as BASE64};
use serde::Serialize;
use rand::RngCore;

use crate::error::{AppError, AppResult};
use crate::kafka::messages::StudentFields;

pub fn encrypt_payload<T: Serialize>(payload: &T, key: &[u8]) -> AppResult<String> {
    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("invalid key: {}", e)))?;
    
    let mut nonce_bytes = [0u8; 12];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);
    
    let plaintext = serde_json::to_vec(payload)
        .map_err(|e| AppError::Encryption(format!("serialization failed: {}", e)))?;
    
    let ciphertext = cipher
        .encrypt(nonce, plaintext.as_ref())
        .map_err(|e| AppError::Encryption(format!("encryption failed: {}", e)))?;
    
    let mut result = nonce_bytes.to_vec();
    result.extend(ciphertext);
    
    Ok(BASE64.encode(&result))
}

pub fn decrypt_payload(encrypted: &str, key: &[u8]) -> AppResult<StudentFields> {
    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("invalid key: {}", e)))?;
    
    let decoded = BASE64
        .decode(encrypted)
        .map_err(|e| AppError::Encryption(format!("base64 decode failed: {}", e)))?;
    
    if decoded.len() < 12 {
        return Err(AppError::Encryption("ciphertext too short".to_string()));
    }
    
    let (nonce_bytes, ciphertext) = decoded.split_at(12);
    let nonce = Nonce::from_slice(nonce_bytes);
    
    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| AppError::Encryption(format!("decryption failed: {}", e)))?;
    
    let student: StudentFields = serde_json::from_slice(&plaintext)
        .map_err(|e| AppError::Encryption(format!("deserialization failed: {}", e)))?;
    
    Ok(student)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_key() -> Vec<u8> {
        let mut key = [0u8; 32];
        OsRng.fill_bytes(&mut key);
        key.to_vec()
    }

    #[test]
    fn test_encrypt_decrypt_roundtrip() {
        let key = test_key();
        let student = StudentFields {
            full_name: "Ivan Ivanov".to_string(),
            diploma_number: "ДВС-2024-001234".to_string(),
            specialty: "Computer Science".to_string(),
            degree: "Бакалавр".to_string(),
            year: 2024,
            faculty: "Faculty of CS".to_string(),
        };

        let encrypted = encrypt_payload(&student, &key).expect("encryption failed");
        let decrypted: StudentFields = decrypt_payload(&encrypted, &key).expect("decryption failed");

        assert_eq!(decrypted.full_name, student.full_name);
        assert_eq!(decrypted.diploma_number, student.diploma_number);
        assert_eq!(decrypted.specialty, student.specialty);
        assert_eq!(decrypted.degree, student.degree);
        assert_eq!(decrypted.year, student.year);
        assert_eq!(decrypted.faculty, student.faculty);
    }

    #[test]
    fn test_encrypt_produces_different_ciphertext() {
        let key = test_key();
        let student = StudentFields {
            full_name: "Test".to_string(),
            diploma_number: "123".to_string(),
            specialty: "Test".to_string(),
            degree: "Test".to_string(),
            year: 2024,
            faculty: "Test".to_string(),
        };

        let encrypted1 = encrypt_payload(&student, &key).expect("encryption failed");
        let encrypted2 = encrypt_payload(&student, &key).expect("encryption failed");

        assert_ne!(encrypted1, encrypted2);
    }

    #[test]
    fn test_decrypt_wrong_key_fails() {
        let key1 = test_key();
        let mut key2 = [0u8; 32];
        OsRng.fill_bytes(&mut key2);

        let student = StudentFields {
            full_name: "Test".to_string(),
            diploma_number: "123".to_string(),
            specialty: "Test".to_string(),
            degree: "Test".to_string(),
            year: 2024,
            faculty: "Test".to_string(),
        };

        let encrypted = encrypt_payload(&student, &key1).expect("encryption failed");
        let result = decrypt_payload(&encrypted, &key2);

        assert!(result.is_err());
    }
}
