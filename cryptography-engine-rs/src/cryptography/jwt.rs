//! JWT token generation and verification using Ed25519 (EdDSA algorithm).
//!
//! This module provides JWT functionality for diploma QR codes using Ed25519
//! elliptic curve signatures, which are more efficient and secure than RSA.
//!
//! The JWT structure follows the secure pattern:
//! ```json
//! {
//!   "sub": "<diploma_hash>",
//!   "diploma_hash": "<diploma_hash>",
//!   "vuz_id": "<uuid>",
//!   "enc": "base64(aes256gcm_ciphertext)",
//!   "iat": 1710000000
//! }
//! ```
//!
//! The `enc` field contains A256GCM-encrypted student data, not plain claims.

use jsonwebtoken::{encode, decode, Header, Algorithm, EncodingKey, DecodingKey, Validation};
use serde::{Serialize, Deserialize};
use tracing::debug;
use uuid::Uuid;
use crate::error::{AppError, AppResult};
use base64::Engine as _;
use base64::engine::general_purpose::STANDARD as BASE64;

use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use rand::RngCore;

/// Student data that will be encrypted and stored in the `enc` claim.
///
/// This struct contains all sensitive student information that should not
/// be visible in plain text when the JWT is decoded.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct EncryptedStudentData {
    /// Diploma number (e.g., "ДИП-123456")
    pub diploma_number: String,
    /// Full name of the student
    pub full_name: String,
    /// Specialty/program name
    pub specialty: String,
    /// Degree
    pub degree: String,
    /// Faculty
    pub faculty: String,
    /// Year of graduation
    pub year: u16,
    /// Random salt used for hash generation
    pub salt: String,
}

/// Claims embedded in the QR code JWT for diploma verification.
///
/// This structure contains minimal metadata with encrypted student data.
/// The actual student information is encrypted in the `enc` field using
/// A256GCM encryption.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QrClaims {
    /// Subject - the diploma hash (used for lookup)
    pub sub: String,
    /// Diploma hash (same as sub for compatibility)
    pub diploma_hash: String,
    /// University ID (UUID)
    pub vuz_id: Uuid,
    /// Encrypted student data (base64-encoded A256GCM ciphertext)
    /// Contains EncryptedStudentData encrypted with PAYLOAD_SECRET
    pub enc: String,
    /// Issued at timestamp
    pub iat: u64,
}

impl QrClaims {
    /// Creates a new QrClaims instance with the current timestamp.
    ///
    /// # Arguments
    /// * `diploma_hash` - The hash of the diploma
    /// * `vuz_id` - The university ID (UUID)
    /// * `encrypted_data` - Base64-encoded A256GCM encrypted student data
    ///
    /// # Returns
    /// A new QrClaims instance with `iat` set to current time
    pub fn new(diploma_hash: String, vuz_id: Uuid, encrypted_data: String) -> Self {
        Self {
            sub: diploma_hash.clone(),
            diploma_hash,
            vuz_id,
            enc: encrypted_data,
            iat: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs(),
        }
    }
}

/// Encrypts student data using A256GCM.
///
/// # Arguments
/// * `data` - The student data to encrypt
/// * `key` - 32-byte symmetric key for encryption (from PAYLOAD_SECRET)
///
/// # Returns
/// Base64-encoded ciphertext (nonce + encrypted data)
///
/// # Format
/// The returned string contains:
/// - 12 bytes: random nonce
/// - N bytes: A256GCM ciphertext with authentication tag
/// All base64-encoded.
fn encrypt_student_data(data: &EncryptedStudentData, key: &[u8]) -> AppResult<String> {
    if key.len() != 32 {
        return Err(AppError::Encryption("key must be 32 bytes for A256GCM".to_string()));
    }

    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("invalid key: {}", e)))?;

    // Generate random nonce (12 bytes for GCM)
    let mut nonce_bytes = [0u8; 12];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    // Serialize student data to JSON
    let plaintext = serde_json::to_vec(data)
        .map_err(|e| AppError::Encryption(format!("serialization failed: {}", e)))?;

    // Encrypt
    let ciphertext = cipher
        .encrypt(nonce, plaintext.as_ref())
        .map_err(|e| AppError::Encryption(format!("encryption failed: {}", e)))?;

    // Combine nonce + ciphertext and base64 encode
    let mut result = nonce_bytes.to_vec();
    let ciphertext_len = ciphertext.len();  // Store length before moving
    result.extend(ciphertext);

    let encoded = BASE64.encode(&result);
    
    // DEBUG: Log encryption details to diagnose potential corruption
    debug!(
        nonce_len = 12,
        ciphertext_len = ciphertext_len,
        total_bytes = result.len(),
        encoded_len = encoded.len(),
        encoded_preview = %&encoded[..encoded.len().min(50)],
        has_plus_chars = encoded.contains('+'),
        has_slash_chars = encoded.contains('/'),
        has_padding = encoded.ends_with('='),
        "encrypt_student_data: base64 encoding complete"
    );

    Ok(encoded)
}

/// Decrypts student data that was encrypted with A256GCM.
///
/// # Arguments
/// * `encrypted` - Base64-encoded ciphertext (nonce + encrypted data)
/// * `key` - 32-byte symmetric key for decryption
///
/// # Returns
/// The decrypted EncryptedStudentData
fn decrypt_student_data(encrypted: &str, key: &[u8]) -> AppResult<EncryptedStudentData> {
    // DEBUG: Log incoming encrypted data to diagnose corruption
    debug!(
        encrypted_len = encrypted.len(),
        encrypted_preview = %&encrypted[..encrypted.len().min(50)],
        has_plus_chars = encrypted.contains('+'),
        has_slash_chars = encrypted.contains('/'),
        has_padding = encrypted.ends_with('='),
        has_space = encrypted.contains(' '),
        has_minus_chars = encrypted.contains('-'),
        has_underscore_chars = encrypted.contains('_'),
        "decrypt_student_data: received encrypted string"
    );

    if key.len() != 32 {
        return Err(AppError::Encryption("key must be 32 bytes for A256GCM".to_string()));
    }

    let cipher = Aes256Gcm::new_from_slice(key)
        .map_err(|e| AppError::Encryption(format!("invalid key: {}", e)))?;

    // Base64 decode
    let decoded = BASE64
        .decode(encrypted)
        .map_err(|e| {
            // DEBUG: Log the exact error and input that caused it
            debug!(
                error = %e,
                encrypted_len = encrypted.len(),
                encrypted_value = %encrypted,
                "decrypt_student_data: base64 decode failed"
            );
            AppError::Encryption(format!("base64 decode failed: {}", e))
        })?;

    // Need at least 12 bytes for nonce + some ciphertext
    if decoded.len() < 12 {
        return Err(AppError::Encryption("ciphertext too short".to_string()));
    }

    // Split nonce and ciphertext
    let (nonce_bytes, ciphertext) = decoded.split_at(12);
    let nonce = Nonce::from_slice(nonce_bytes);

    // Decrypt
    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| AppError::Encryption(format!("decryption failed: {}", e)))?;

    // Deserialize
    let data: EncryptedStudentData = serde_json::from_slice(&plaintext)
        .map_err(|e| AppError::Encryption(format!("deserialization failed: {}", e)))?;

    Ok(data)
}

/// Builds QrClaims with encrypted student data.
///
/// This is the main function for creating QR code claims. It:
/// 1. Creates EncryptedStudentData from the provided fields
/// 2. Encrypts the data with A256GCM using the payload secret
/// 3. Base64-encodes the ciphertext
/// 4. Returns QrClaims with the encrypted content
///
/// # Arguments
/// * `diploma_hash` - The computed diploma hash
/// * `vuz_id` - University UUID
/// * `diploma_number` - Diploma number
/// * `full_name` - Full student name
/// * `specialty` - Specialty/program name
/// * `year` - Graduation year
/// * `salt` - Random salt used for hash
/// * `payload_secret` - 32-byte encryption key
///
/// # Returns
/// QrClaims ready to be signed as a JWT
///
/// # Example
/// ```ignore
/// let claims = build_qr_claims(
///     "abc123hash".to_string(),
///     vuz_id,
///     "ДИП-123456".to_string(),
///     "Иванов Иван Иванович".to_string(),
///     "Программная инженерия".to_string(),
///     2024,
///     "randomsalt".to_string(),
///     &payload_secret,
/// )?;
/// let token = create_qr_jwt(&claims, &private_key_pem)?;
/// ```
pub fn build_qr_claims(
    diploma_hash: String,
    vuz_id: Uuid,
    diploma_number: String,
    full_name: String,
    specialty: String,
    degree: String,
    faculty: String,
    year: u16,
    salt: String,
    payload_secret: &[u8],
) -> AppResult<QrClaims> {
    // Build encrypted data struct
    let encrypted_data = EncryptedStudentData {
        diploma_number,
        full_name,
        specialty,
        degree,
        faculty,
        year,
        salt,
    };

    // Encrypt with A256GCM
    let enc = encrypt_student_data(&encrypted_data, payload_secret)?;

    // Build claims with current timestamp
    Ok(QrClaims::new(diploma_hash, vuz_id, enc))
}

/// Decrypts the student data from QrClaims.
///
/// This function is used by the verification service to decrypt
/// the student data from a verified JWT.
///
/// # Arguments
/// * `claims` - The verified QrClaims
/// * `payload_secret` - 32-byte decryption key
///
/// # Returns
/// The decrypted EncryptedStudentData
///
/// # Example
/// ```ignore
/// let claims = verify_qr_jwt(&token, &public_key_pem)?;
/// let student_data = decrypt_qr_claims(&claims, &payload_secret)?;
/// println!("Student: {}", student_data.full_name);
/// ```
pub fn decrypt_qr_claims(claims: &QrClaims, payload_secret: &[u8]) -> AppResult<EncryptedStudentData> {
    decrypt_student_data(&claims.enc, payload_secret)
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

    // DEBUG: Log claims before JWT encoding to diagnose enc field corruption
    debug!(
        sub = %claims.sub,
        vuz_id = %claims.vuz_id,
        enc_len = claims.enc.len(),
        enc_preview = %&claims.enc[..claims.enc.len().min(50)],
        enc_full = %claims.enc,  // LOG FULL ENC VALUE
        enc_has_plus = claims.enc.contains('+'),
        enc_has_slash = claims.enc.contains('/'),
        enc_has_padding = claims.enc.ends_with('='),
        iat = claims.iat,
        "create_qr_jwt: claims before encoding"
    );

    // DEBUG: Manually serialize claims to JSON to check for corruption
    let claims_json = serde_json::to_string(claims)
        .map_err(|e| AppError::Jwt(format!("failed to serialize claims to JSON: {}", e)))?;
    debug!(
        json_len = claims_json.len(),
        json_preview = %&claims_json[..claims_json.len().min(200)],
        "create_qr_jwt: manually serialized claims JSON"
    );

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

    // DEBUG: Log decoded claims to diagnose enc field corruption
    debug!(
        sub = %decoded.claims.sub,
        vuz_id = %decoded.claims.vuz_id,
        enc_len = decoded.claims.enc.len(),
        enc_preview = %&decoded.claims.enc[..decoded.claims.enc.len().min(50)],
        enc_has_plus = decoded.claims.enc.contains('+'),
        enc_has_slash = decoded.claims.enc.contains('/'),
        enc_has_minus = decoded.claims.enc.contains('-'),
        enc_has_underscore = decoded.claims.enc.contains('_'),
        enc_has_space = decoded.claims.enc.contains(' '),
        "verify_qr_jwt: decoded claims"
    );

    Ok(decoded.claims)
}

/// Builds the verification URL for a QR code token.
pub fn build_qr_url(token: &str, config: &crate::config::AppConfig) -> String {
    format!("{}/{}", config.app.verification_base_url, token)
}

// ============================================================================
// Legacy ServiceClaims - kept for backward compatibility
// ============================================================================

/// Service JWT claims for internal service tokens.
/// These tokens are encrypted using JWE with A256GCM algorithm.
///
/// **Note:** This is kept for backward compatibility. For new code,
/// use QrClaims with the build_qr_claims() function.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceClaims {
    /// Subject - the diploma hash
    pub sub: String,
    /// Diploma hash (same as sub for compatibility)
    pub diploma_hash: String,
    /// University ID (UUID)
    pub vuz_id: Uuid,
    /// Encrypted content (base64-encoded A256GCM ciphertext)
    pub enc: String,
    /// Issued at timestamp
    pub iat: u64,
}

impl ServiceClaims {
    /// Creates a new ServiceClaims instance with the current timestamp.
    #[deprecated(note = "Use QrClaims with build_qr_claims() instead")]
    pub fn new(diploma_hash: String, vuz_id: Uuid, encrypted_content: String) -> Self {
        Self {
            sub: diploma_hash.clone(),
            diploma_hash,
            vuz_id,
            enc: encrypted_content,
            iat: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs(),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::net::ToSocketAddrs;
    use serde_json::to_string;
    use super::*;

    fn test_key() -> Vec<u8> {
        let mut key = [0u8; 32];
        OsRng.fill_bytes(&mut key);
        key.to_vec()
    }

    #[test]
    fn test_encrypt_decrypt_student_data() {
        let key = test_key();
        let data = EncryptedStudentData {
            diploma_number: "ДИП-123456".to_string(),
            full_name: "Иванов Иван Иванович".to_string(),
            specialty: "Программная инженерия".to_string(),
            degree: "Test".to_string(),
            faculty: "test".to_string(),
            year: 2024,
            salt: "randomsalt123".to_string(),
        };

        let encrypted = encrypt_student_data(&data, &key).expect("encryption failed");
        let decrypted = decrypt_student_data(&encrypted, &key).expect("decryption failed");

        assert_eq!(decrypted, data);
    }

    #[test]
    fn test_encrypt_produces_different_ciphertext() {
        let key = test_key();
        let data = EncryptedStudentData {
            diploma_number: "123".to_string(),
            full_name: "Test".to_string(),
            specialty: "Test".to_string(),
            degree: "test".to_string(),
            faculty: "Factulltye".to_string(),
            year: 2024,
            salt: "salt".to_string(),
        };

        let encrypted1 = encrypt_student_data(&data, &key).expect("encryption failed");
        let encrypted2 = encrypt_student_data(&data, &key).expect("encryption failed");

        // Different due to random nonce
        assert_ne!(encrypted1, encrypted2);
    }

    #[test]
    fn test_decrypt_wrong_key_fails() {
        let key1 = test_key();
        let key2 = test_key(); // Different key

        let data = EncryptedStudentData {
            diploma_number: "123".to_string(),
            full_name: "Test".to_string(),
            specialty: "Test".to_string(),
            degree: "AAAAA".to_string(),
            faculty: "FFFFF".to_string(),
            year: 2024,
            salt: "salt".to_string(),
        };

        let encrypted = encrypt_student_data(&data, &key1).expect("encryption failed");
        let result = decrypt_student_data(&encrypted, &key2);

        assert!(result.is_err());
    }

    #[test]
    fn test_decrypt_tampered_fails() {
        let key = test_key();
        let data = EncryptedStudentData {
            diploma_number: "123".to_string(),
            full_name: "Test".to_string(),
            specialty: "Test".to_string(),
            degree: "AAafaf".to_string(),
            faculty: "TestFaculty".to_string(),
            year: 2024,
            salt: "salt".to_string(),
        };

        let encrypted = encrypt_student_data(&data, &key).expect("encryption failed");

        // Tamper with the encrypted data
        let mut tampered = encrypted.clone();
        let mid = tampered.len() / 2;
        let chars: Vec<char> = tampered.chars().collect();
        if chars[mid] == 'a' {
            tampered.replace_range(mid..mid+1, "b");
        } else {
            tampered.replace_range(mid..mid+1, "a");
        }

        let result = decrypt_student_data(&tampered, &key);
        assert!(result.is_err());
    }

    #[test]
    fn test_encrypt_invalid_key_length() {
        let short_key = [0u8; 16]; // Too short - needs 32 bytes

        let data = EncryptedStudentData {
            diploma_number: "123".to_string(),
            full_name: "Test".to_string(),
            specialty: "Test".to_string(),
            degree: "afd".to_string(),
            faculty: "afa".to_string(),
            year: 2024,
            salt: "salt".to_string(),
        };

        let result = encrypt_student_data(&data, &short_key);
        assert!(result.is_err());
    }

    #[test]
    fn test_build_qr_claims() {
        let key = test_key();
        let vuz_id = Uuid::parse_str("550e8400-e29b-41d4-a716-446655440000").unwrap();

        let claims = build_qr_claims(
            "test-hash-123".to_string(),
            vuz_id,
            "ДИП-123456".to_string(),
            "Иванов Иван Иванович".to_string(),
            "Программная инженерия".to_string(),
            2024,
            "randomsalt".to_string(),
            &key,
        ).expect("build_qr_claims failed");

        assert_eq!(claims.sub, "test-hash-123");
        assert_eq!(claims.diploma_hash, "test-hash-123");
        assert_eq!(claims.vuz_id, vuz_id);
        assert!(!claims.enc.is_empty());
        assert!(claims.iat > 0);
    }

    #[test]
    fn test_decrypt_qr_claims() {
        let key = test_key();
        let vuz_id = Uuid::nil();

        let claims = build_qr_claims(
            "hash".to_string(),
            vuz_id,
            "ДИП-123".to_string(),
            "Test Student".to_string(),
            "Computer Science".to_string(),
            2024,
            "salt123".to_string(),
            &key,
        ).expect("build_qr_claims failed");

        let decrypted = decrypt_qr_claims(&claims, &key).expect("decrypt_qr_claims failed");

        assert_eq!(decrypted.diploma_number, "ДИП-123");
        assert_eq!(decrypted.full_name, "Test Student");
        assert_eq!(decrypted.specialty, "Computer Science");
        assert_eq!(decrypted.year, 2024);
        assert_eq!(decrypted.salt, "salt123");
    }

    #[test]
    fn test_eddsa_jwt_roundtrip() {
        use ed25519_dalek::SigningKey;
        use rand::rngs::OsRng;
        use pkcs8::{EncodePrivateKey, EncodePublicKey};

        // Generate a new Ed25519 keypair
        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();

        // Convert to PKCS#8 PEM format for private key
        let private_key_pem = EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");

        // Convert to PEM format for public key
        let public_key_pem = EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");

        let key = test_key();
        let vuz_id = Uuid::nil();

        let claims = build_qr_claims(
            "test-hash".to_string(),
            vuz_id,
            "TEST-2024-001".to_string(),
            "Test Student".to_string(),
            "Computer Science".to_string(),
            2024,
            "randomsalt".to_string(),
            &key,
        ).expect("build_qr_claims failed");

        // Create JWT
        let token = create_qr_jwt(&claims, &private_key_pem).expect("Failed to create JWT");

        // Verify JWT
        let decoded = verify_qr_jwt(&token, &public_key_pem).expect("Failed to verify JWT");

        assert_eq!(decoded.sub, claims.sub);
        assert_eq!(decoded.diploma_hash, claims.diploma_hash);
        assert_eq!(decoded.vuz_id, claims.vuz_id);

        // Decrypt and verify student data
        let student = decrypt_qr_claims(&decoded, &key).expect("Failed to decrypt");
        assert_eq!(student.full_name, "Test Student");
    }

    #[test]
    fn test_eddsa_jwt_different_keys() {
        use ed25519_dalek::SigningKey;
        use rand::rngs::OsRng;
        use pkcs8::{EncodePrivateKey, EncodePublicKey};

        let mut csprng = OsRng;

        // Generate two different keypairs
        let signing_key1: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key1 = signing_key1.verifying_key();

        let signing_key2: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key2 = signing_key2.verifying_key();

        let private_key_pem1 = EncodePrivateKey::to_pkcs8_pem(&signing_key1, pkcs8::LineEnding::default())
            .expect("Failed to encode private key 1");
        let public_key_pem1 = EncodePublicKey::to_public_key_pem(&verifying_key1, pkcs8::LineEnding::default())
            .expect("Failed to encode public key 1");

        let public_key_pem2 = EncodePublicKey::to_public_key_pem(&verifying_key2, pkcs8::LineEnding::default())
            .expect("Failed to encode public key 2");

        let key = test_key();
        let claims = build_qr_claims(
            "hash".to_string(),
            Uuid::nil(),
            "123".to_string(),
            "Test".to_string(),
            "CS".to_string(),
            2024,
            "salt".to_string(),
            &key,
        ).expect("build_qr_claims failed");

        // Create JWT with key 1
        let token = create_qr_jwt(&claims, &private_key_pem1).expect("Failed to create JWT");

        // Should verify with key 1
        verify_qr_jwt(&token, &public_key_pem1).expect("Failed to verify with correct key");

        // Should fail with key 2
        assert!(verify_qr_jwt(&token, &public_key_pem2).is_err());
    }

    #[test]
    fn test_eddsa_jwt_tampered_token_fails() {
        use ed25519_dalek::SigningKey;
        use rand::rngs::OsRng;
        use pkcs8::{EncodePrivateKey, EncodePublicKey};

        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();

        let private_key_pem = EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        let public_key_pem = EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");

        let key = test_key();
        let claims = build_qr_claims(
            "hash".to_string(),
            Uuid::nil(),
            "123".to_string(),
            "Test".to_string(),
            "CS".to_string(),
            2024,
            "salt".to_string(),
            &key,
        ).expect("build_qr_claims failed");

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

    #[test]
    fn test_qr_claims_structure() {
        // Verify the JWT structure matches the expected format
        let key = test_key();
        let vuz_id = Uuid::parse_str("550e8400-e29b-41d4-a716-446655440000").unwrap();

        let claims = build_qr_claims(
            "test-diploma-hash".to_string(),
            vuz_id,
            "ДИП-654321".to_string(),
            "Петров Петр Петрович".to_string(),
            "Информационные технологии".to_string(),
            2025,
            "mysalt".to_string(),
            &key,
        ).expect("build_qr_claims failed");

        // Serialize to JSON to verify structure
        let json = serde_json::to_string(&claims).expect("serialization failed");

        // Verify required fields are present
        assert!(json.contains("\"sub\":\"test-diploma-hash\""));
        assert!(json.contains("\"diploma_hash\":\"test-diploma-hash\""));
        assert!(json.contains("\"vuz_id\":\"550e8400-e29b-41d4-a716-446655440000\""));
        assert!(json.contains("\"enc\":\""));
        assert!(json.contains("\"iat\":"));

        // Verify plain student data is NOT present
        assert!(!json.contains("Петров"));
        assert!(!json.contains("full_name"));
        assert!(!json.contains("diploma_number"));
    }

    /// End-to-end test simulating the complete flow:
    /// 1. Build QR claims with encrypted student data
    /// 2. Create JWT token
    /// 3. Serialize to JSON (simulating Kafka output)
    /// 4. Deserialize from JSON
    /// 5. Verify JWT signature
    /// 6. Decrypt student data
    #[test]
    fn test_end_to_end_flow_with_json_serialization() {
        use ed25519_dalek::SigningKey;
        use rand::rngs::OsRng;
        use pkcs8::{EncodePrivateKey, EncodePublicKey};

        // Generate Ed25519 keypair
        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();

        let private_key_pem = EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        let public_key_pem = EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");

        // Setup
        let key = test_key();
        let vuz_id = Uuid::nil();

        // Step 1: Build QR claims
        let original_student = EncryptedStudentData {
            diploma_number: "ДИП-2024-001234".to_string(),
            full_name: "Иванов Иван Иванович".to_string(),
            specialty: "Программная инженерия".to_string(),
            degree: "aGGGa".to_string(),
            faculty: "AFagga".to_string(),
            year: 2024,
            salt: "randomsalt123".to_string(),
        };

        let claims = build_qr_claims(
            "test-hash-e2e".to_string(),
            vuz_id,
            original_student.diploma_number.clone(),
            original_student.full_name.clone(),
            original_student.specialty.clone(),
            original_student.degree.clone(),
            original_student.degree.clone(),
            original_student.year,
            original_student.salt.clone(),
            &key,
        ).expect("build_qr_claims failed");

        // Step 2: Create JWT
        let jwt_token = create_qr_jwt(&claims, &private_key_pem).expect("create_qr_jwt failed");

        // Step 3: Simulate Kafka JSON serialization
        #[derive(Serialize, Deserialize)]
        struct KafkaMessage {
            qr_payload: String,
        }
        let kafka_msg = KafkaMessage { qr_payload: jwt_token };
        let kafka_json = serde_json::to_string(&kafka_msg).expect("kafka serialization failed");
        
        // Verify the enc field survives JSON serialization
        assert!(kafka_json.contains("qr_payload"));
        
        // Step 4: Deserialize from JSON (simulating consumer)
        let deserialized: KafkaMessage = serde_json::from_str(&kafka_json).expect("kafka deserialization failed");
        let received_token = deserialized.qr_payload;

        // Step 5: Verify JWT signature and decode
        let decoded_claims = verify_qr_jwt(&received_token, &public_key_pem)
            .expect("JWT verification failed");

        // Step 6: Decrypt student data
        let decrypted_student = decrypt_qr_claims(&decoded_claims, &key)
            .expect("decrypt_qr_claims failed");

        // Verify all fields match
        assert_eq!(decrypted_student.diploma_number, original_student.diploma_number);
        assert_eq!(decrypted_student.full_name, original_student.full_name);
        assert_eq!(decrypted_student.specialty, original_student.specialty);
        assert_eq!(decrypted_student.year, original_student.year);
        assert_eq!(decrypted_student.salt, original_student.salt);
    }

    /// Test that base64 special characters (+, /) in encrypted data survive the full flow
    #[test]
    fn test_base64_special_characters_survive_roundtrip() {
        use ed25519_dalek::SigningKey;
        use rand::rngs::OsRng;
        use pkcs8::{EncodePrivateKey, EncodePublicKey};

        let mut csprng = OsRng;
        let signing_key: SigningKey = SigningKey::generate(&mut csprng);
        let verifying_key = signing_key.verifying_key();

        let private_key_pem = EncodePrivateKey::to_pkcs8_pem(&signing_key, pkcs8::LineEnding::default())
            .expect("Failed to encode private key");
        let public_key_pem = EncodePublicKey::to_public_key_pem(&verifying_key, pkcs8::LineEnding::default())
            .expect("Failed to encode public key");

        let key = test_key();

        // Create multiple claims to increase chance of hitting base64 special chars
        for i in 0..10 {
            let claims = build_qr_claims(
                format!("hash-{}", i),
                Uuid::nil(),
                format!("DIP-{}", i),
                format!("Student {}", i),
                "Test Specialty".to_string(),
                2024,
                format!("salt-{}", i),
                &key,
            ).expect("build_qr_claims failed");

            // Check if this encryption contains special characters
            let has_plus = claims.enc.contains('+');
            let has_slash = claims.enc.contains('/');
            
            // Create and verify JWT
            let token = create_qr_jwt(&claims, &private_key_pem).expect("create_qr_jwt failed");
            let decoded = verify_qr_jwt(&token, &public_key_pem).expect("verify_qr_jwt failed");
            
            // Verify enc field is preserved
            assert_eq!(decoded.enc, claims.enc,
                "enc field mismatch! Original contains +:{} /:{}, decoded contains +:{} /:{}",
                has_plus, has_slash,
                decoded.enc.contains('+'), decoded.enc.contains('/'));

            // Verify decryption works
            let decrypted = decrypt_qr_claims(&decoded, &key).expect("decrypt_qr_claims failed");
            assert_eq!(decrypted.diploma_number, format!("DIP-{}", i));
        }
    }
}
