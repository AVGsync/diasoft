use sha2::{Digest, Sha256};
use hex;
use rand::RngCore;
use rand::rngs::OsRng;
use uuid::Uuid;

use crate::error::{AppError, AppResult};

pub struct StudentFieldsForHash<'a> {
    pub full_name: &'a str,
    pub diploma_number: &'a str,
    pub specialty: &'a str,
    pub degree: &'a str,
    pub faculty: &'a str,
    pub year: u16,
}

pub fn generate_salt() -> AppResult<String> {
    let mut bytes = [0u8; 32];
    OsRng.try_fill_bytes(&mut bytes)
        .map_err(|e| AppError::Hashing(format!("failed to generate salt: {}", e)))?;
    Ok(hex::encode(bytes))
}

pub fn hash_diploma(student: &StudentFieldsForHash, vuz_id: Uuid, salt: &str) -> AppResult<String> {
    let canonical = format!(
        "{}|{}|{}|{}|{}|{}",
        student.full_name,
        student.diploma_number,
        student.specialty,
        student.year,
        vuz_id,
        salt
    );

    let mut hasher = Sha256::new();
    hasher.update(canonical.as_bytes());
    let result = hasher.finalize();
    Ok(hex::encode(result))
}

pub fn hash_sha256<T: ToString>(data: T) -> String {
    let formatted = data.to_string();
    let mut hasher = Sha256::new();
    hasher.update(formatted.as_bytes());
    let result = hasher.finalize();
    hex::encode(result)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hash_sha256_empty_string() {
        let hash = hash_sha256("");
        assert_eq!(
            hash,
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        );
    }

    #[test]
    fn test_hash_sha256_known_value() {
        let hash = hash_sha256("hello world");
        assert_eq!(
            hash,
            "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
        );
    }

    #[test]
    fn test_hash_sha256_returns_64_chars() {
        let hash = hash_sha256("any input");
        assert_eq!(hash.len(), 64);
    }

    #[test]
    fn test_hash_sha256_is_deterministic() {
        let hash1 = hash_sha256("test");
        let hash2 = hash_sha256("test");
        assert_eq!(hash1, hash2);
    }

    #[test]
    fn test_combined_workflow() {
        let data = "important data";
        let hash = hash_sha256(&data);
        assert_eq!(hash.len(), 64);
        assert!(hash.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn test_generate_salt() {
        let salt = generate_salt().expect("salt generation failed");
        assert_eq!(salt.len(), 64);
        assert!(salt.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn test_generate_salt_unique() {
        let salt1 = generate_salt().expect("salt generation failed");
        let salt2 = generate_salt().expect("salt generation failed");
        assert_ne!(salt1, salt2);
    }

    #[test]
    fn test_hash_diploma() {
        let vuz_id = uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap();
        let student = StudentFieldsForHash {
            full_name: "Ivan Ivanov",
            diploma_number: "ДВС-2024-001234",
            specialty: "Computer Science",
            degree: "aAafag",
            faculty: "Agag",
            year: 2024,
        };
        let salt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
        
        let hash = hash_diploma(&student, vuz_id, salt).expect("hash failed");
        
        assert_eq!(hash.len(), 64);
        assert!(hash.chars().all(|c| c.is_ascii_hexdigit()));
    }
}
