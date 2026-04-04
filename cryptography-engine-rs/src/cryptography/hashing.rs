use sha2::{Digest, Sha256};
use hex;
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

pub fn derive_salt(vuz_id: Uuid, diploma_number: &str) -> AppResult<String> {
    let trimmed = diploma_number.trim();
    if trimmed.is_empty() {
        return Err(AppError::Hashing("diploma number must not be empty".to_string()));
    }

    Ok(hash_sha256(format!("diasoft|{}|{}", vuz_id, trimmed)))
}

pub fn hash_diploma(student: &StudentFieldsForHash, vuz_id: Uuid, salt: &str) -> AppResult<String> {
    let canonical = format!(
        "{}|{}|{}|{}|{}|{}|{}|{}",
        student.diploma_number,
        student.full_name,
        student.specialty,
        student.degree,
        student.faculty,
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
    fn test_derive_salt() {
        let salt = derive_salt(
            uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap(),
            "DIP-2024-0001",
        ).expect("salt derivation failed");
        assert_eq!(salt.len(), 64);
        assert!(salt.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn test_derive_salt_is_deterministic() {
        let vuz_id = uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap();
        let salt1 = derive_salt(vuz_id, "DIP-2024-0001").expect("salt derivation failed");
        let salt2 = derive_salt(vuz_id, "DIP-2024-0001").expect("salt derivation failed");
        let salt3 = derive_salt(vuz_id, "DIP-2024-0002").expect("salt derivation failed");

        assert_eq!(salt1, salt2);
        assert_ne!(salt1, salt3);
    }

    #[test]
    fn test_hash_diploma() {
        let vuz_id = uuid::Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap();
        let student = StudentFieldsForHash {
            full_name: "Ivan Ivanov",
            diploma_number: "ДВС-2024-001234",
            specialty: "Computer Science",
            degree: "Bachelor",
            faculty: "FKN",
            year: 2024,
        };
        let salt = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
        
        let hash = hash_diploma(&student, vuz_id, salt).expect("hash failed");
        
        assert_eq!(hash.len(), 64);
        assert!(hash.chars().all(|c| c.is_ascii_hexdigit()));
    }
}
