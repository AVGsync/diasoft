//! Diploma hashing operations.
//!
//! Implements the canonical diploma hashing algorithm:
//! ```text
//! salt = random 32 bytes → hex string (64 chars)
//! raw  = "{full_name}|{diploma_number}|{specialty}|{year}|{vuz_id}|{salt}"
//! hash = SHA-256(raw) → 64-char hex string
//! ```
//!
//! The salt is stored inside the QR JWT payload so the verifier service
//! can independently recompute the hash from the scanned JWT without
//! any additional DB lookup.

use sha2::{Sha256, Digest};
use rand::RngCore;
use uuid::Uuid;

use crate::error::{AppError, AppResult};

/// Student fields required for diploma hashing
#[derive(Debug, Clone)]
pub struct StudentFieldsForHash<'a> {
    pub full_name: &'a str,
    pub diploma_number: &'a str,
    pub specialty: &'a str,
    pub year: u16,
}

/// Generates a cryptographically random 32-byte salt as a hex string (64 characters).
///
/// Uses `rand::thread_rng()` for cryptographically secure random number generation.
pub fn generate_salt() -> AppResult<String> {
    let mut salt_bytes = [0u8; 32];
    rand::thread_rng()
        .try_fill_bytes(&mut salt_bytes)
        .map_err(|e| AppError::Hashing(format!("failed to generate salt: {}", e)))?;
    Ok(hex::encode(salt_bytes))
}

/// Computes the SHA-256 hash of a diploma record.
///
/// The hash is computed over the canonical string representation:
/// `{full_name}|{diploma_number}|{specialty}|{year}|{vuz_id}|{salt}`
///
/// # Arguments
/// * `student` - Student fields for hashing
/// * `vuz_id` - University UUID
/// * `salt` - 64-character hex salt string
///
/// # Returns
/// 64-character hex-encoded SHA-256 hash
pub fn hash_diploma(
    student: &StudentFieldsForHash<'_>,
    vuz_id: Uuid,
    salt: &str,
) -> AppResult<String> {
    // Build canonical string for hashing
    let canonical = format!(
        "{}|{}|{}|{}|{}|{}",
        student.full_name,
        student.diploma_number,
        student.specialty,
        student.year,
        vuz_id,
        salt
    );
    
    // Compute SHA-256 hash
    let mut hasher = Sha256::new();
    hasher.update(canonical.as_bytes());
    let hash_bytes = hasher.finalize();
    
    Ok(hex::encode(hash_bytes))
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_generate_salt_length() {
        let salt = generate_salt().unwrap();
        assert_eq!(salt.len(), 64);
        assert!(salt.chars().all(|c| c.is_ascii_hexdigit()));
    }
    
    #[test]
    fn test_hash_diploma_deterministic() {
        let student = StudentFieldsForHash {
            full_name: "Иванов Иван Иванович",
            diploma_number: "ДВС-2024-001234",
            specialty: "Программная инженерия",
            year: 2024,
        };
        let vuz_id = Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap();
        let salt = "a".repeat(64);
        
        let hash1 = hash_diploma(&student, vuz_id, &salt).unwrap();
        let hash2 = hash_diploma(&student, vuz_id, &salt).unwrap();
        
        assert_eq!(hash1, hash2);
        assert_eq!(hash1.len(), 64);
    }
    
    #[test]
    fn test_hash_diploma_different_salt() {
        let student = StudentFieldsForHash {
            full_name: "Иванов Иван Иванович",
            diploma_number: "ДВС-2024-001234",
            specialty: "Программная инженерия",
            year: 2024,
        };
        let vuz_id = Uuid::parse_str("00000000-0000-0000-0000-000000000001").unwrap();
        
        let hash1 = hash_diploma(&student, vuz_id, &"a".repeat(64)).unwrap();
        let hash2 = hash_diploma(&student, vuz_id, &"b".repeat(64)).unwrap();
        
        assert_ne!(hash1, hash2);
    }
}
