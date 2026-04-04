//! Diploma processing orchestrator for Kafka messages.
//!
//! This module provides the [`DiplomaProcessor`] which coordinates the complete
//! diploma processing pipeline: hashing, signing, encryption, QR JWT generation,
//! and result publishing.

use std::sync::Arc;

use chrono::Utc;
use sqlx::PgPool;
use tracing::{debug, info, warn, error};

use crate::config::AppConfig;
use crate::cryptography::{
    create_qr_jwt, encrypt_payload, generate_salt, hash_diploma,
    sign_hash, StudentFieldsForHash, QrClaims,
};
use crate::db::models::NewDiplomaHash;
use crate::db::repository::{diploma_hash_exists, get_university_key, insert_diploma_hash};
use crate::error::{AppError, AppResult};
use crate::kafka::messages::{DiplomaTask, ProcessingResult, ProcessingStatus};
use crate::kafka::producer::KafkaProducer;

/// Orchestrates the complete diploma processing pipeline.
///
/// The processor receives [`DiplomaTask`] messages and transforms them into
/// [`ProcessingResult`] messages by executing the following steps:
///
/// 1. Fetch the university's encrypted private key from the database
/// 2. Generate a salt and compute the diploma hash
/// 3. Sign the hash with the university's private key
/// 4. Encrypt the student payload
/// 5. Generate a QR JWT for verification
/// 6. Persist the diploma hash to the database
/// 7. Send the processing result to the output Kafka topic
#[derive(Clone)]
pub struct DiplomaProcessor {
    config: Arc<AppConfig>,
    pool: Arc<PgPool>,
    producer: Arc<KafkaProducer>,
}

impl DiplomaProcessor {
    /// Creates a new diploma processor with the given dependencies.
    ///
    /// # Arguments
    ///
    /// * `config` - Application configuration containing encryption keys and URLs
    /// * `pool` - PostgreSQL connection pool for database operations
    /// * `producer` - Kafka producer for sending processing results
    pub fn new(
        config: Arc<AppConfig>,
        pool: Arc<PgPool>,
        producer: Arc<KafkaProducer>,
    ) -> Self {
        Self {
            config,
            pool,
            producer,
        }
    }

    /// Processes a single diploma task through the complete pipeline.
    ///
    /// This method handles all errors internally and sends either a success
    /// or error result to the output Kafka topic. It never returns an error
    /// to prevent message redelivery loops for non-recoverable errors.
    ///
    /// # Arguments
    ///
    /// * `task` - The diploma task received from Kafka
    pub async fn process(&self, task: DiplomaTask) {
        info!(
            batch_id = %task.batch_id,
            record_index = task.record_index,
            vuz_id = %task.vuz_id,
            "Processing diploma task"
        );

        match self.process_inner(&task).await {
            Ok(result) => {
                if result.status == ProcessingStatus::Ok {
                    info!(
                        batch_id = %task.batch_id,
                        record_index = task.record_index,
                        diploma_hash = %result.diploma_hash,
                        "Diploma processed successfully"
                    );
                } else {
                    warn!(
                        batch_id = %task.batch_id,
                        record_index = task.record_index,
                        error = ?result.error,
                        "Diploma processing failed"
                    );
                }
                
                if let Err(e) = self.producer.send_result(&result).await {
                    error!(
                        batch_id = %task.batch_id,
                        record_index = task.record_index,
                        error = %e,
                        "Failed to send processing result to Kafka"
                    );
                }
            }
            Err(e) => {
                error!(
                    batch_id = %task.batch_id,
                    record_index = task.record_index,
                    error = %e,
                    "Unexpected error during diploma processing"
                );
                
                // Send error result
                let error_result = ProcessingResult::error(
                    task.batch_id,
                    task.vuz_id,
                    task.record_index,
                    e.to_string(),
                );
                
                if let Err(e) = self.producer.send_result(&error_result).await {
                    error!(
                        batch_id = %task.batch_id,
                        record_index = task.record_index,
                        error = %e,
                        "Failed to send error result to Kafka"
                    );
                }
            }
        }
    }

    /// Internal processing logic that returns a result.
    async fn process_inner(&self, task: &DiplomaTask) -> AppResult<ProcessingResult> {
        // Step 1: Fetch university key
        let key_data = get_university_key(&self.pool, task.vuz_id).await?;
        debug!(
            vuz_id = %task.vuz_id,
            key_algorithm = %key_data.key_algorithm,
            "Retrieved university signing key"
        );

        // Step 2: Generate salt and hash
        let salt = generate_salt()?;
        let student_for_hash = StudentFieldsForHash {
            full_name: &task.student.full_name,
            diploma_number: &task.student.diploma_number,
            specialty: &task.student.specialty,
            year: task.student.year,
        };
        let diploma_hash = hash_diploma(&student_for_hash, task.vuz_id, &salt)?;
        debug!(diploma_hash = %diploma_hash, "Generated diploma hash");

        // Step 3: Check for duplicates
        if diploma_hash_exists(&self.pool, &diploma_hash).await? {
            warn!(
                diploma_hash = %diploma_hash,
                "Diploma hash already exists, returning existing result"
            );
            // Return success with existing hash (idempotent behavior)
            // QR payload not re-sent for duplicates - use None
            return Ok(ProcessingResult {
                batch_id: task.batch_id,
                vuz_id: task.vuz_id,
                record_index: task.record_index,
                diploma_hash,
                qr_payload: None,
                status: ProcessingStatus::Ok,
                error: None,
                processed_at: Utc::now(),
            });
        }

        // Step 4: Decrypt the private key (using encryption_key from config)
        let private_key_pem = self.decrypt_private_key(&key_data.encrypted_private_key)?;

        // Step 5: Sign the hash
        let signature = sign_hash(&diploma_hash, &private_key_pem)?;
        debug!(signature_len = signature.len(), "Signed diploma hash");

        // Step 6: Encrypt the student payload
        let encryption_key_bytes = self.get_encryption_key_bytes()?;
        let encrypted_payload = encrypt_payload(&task.student, &encryption_key_bytes)?;
        debug!(payload_len = encrypted_payload.len(), "Encrypted student payload");

        // Step 7: Generate QR JWT
        let qr_claims = QrClaims {
            sub: Some(diploma_hash.clone()),
            diploma_hash: Some(diploma_hash.clone()),
            vuz_id: task.vuz_id,
            diploma_number: task.student.diploma_number.clone(),
            student_name: task.student.full_name.clone(),
            specialty: task.student.specialty.clone(),
            year: task.student.year,
            salt: salt.clone(),
            iat: Utc::now().timestamp() as u64,
            exp: None, // No expiration for diploma verification
        };

        let qr_token = create_qr_jwt(&qr_claims, &private_key_pem)?;
        // Send raw JWT token as qr_payload, not a URL
        let qr_payload = qr_token;
        debug!(qr_payload_len = qr_payload.len(), "Generated QR payload");

        // Step 8: Persist to database
        let new_diploma = NewDiplomaHash {
            hash: &diploma_hash,
            vuz_id: task.vuz_id,
            diploma_number: &task.student.diploma_number,
            signature: Some(&signature),
        };
        insert_diploma_hash(&self.pool, &new_diploma).await?;
        debug!(diploma_hash = %diploma_hash, "Persisted diploma hash to database");

        // Step 9: Return success result
        Ok(ProcessingResult::success(
            task.batch_id,
            task.vuz_id,
            task.record_index,
            diploma_hash,
            qr_payload,
        ))
    }

    /// Decrypts the university's private key using the master encryption key.
    /// 
    /// The encrypted key format is: "v1:" + base64(nonce + ciphertext)
    /// The master key is base64-encoded (not hex).
    fn decrypt_private_key(&self, encrypted_key: &str) -> AppResult<String> {
        use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
        use aes_gcm::{
            aead::Aead,
            Aes256Gcm, KeyInit, Nonce,
        };

        let master_key = self.config.app.encryption_key.as_ref()
            .ok_or_else(|| AppError::Encryption("master encryption key not configured".into()))?;

        // Master key is base64-encoded (not hex)
        let key_bytes = BASE64.decode(master_key)
            .map_err(|e| AppError::Encryption(format!("invalid master key format: {}", e)))?;

        if key_bytes.len() != 32 {
            return Err(AppError::Encryption(
                format!("master key must be 32 bytes, got {}", key_bytes.len())
            ));
        }

        let cipher = Aes256Gcm::new_from_slice(&key_bytes)
            .map_err(|e| AppError::Encryption(format!("failed to initialize cipher: {}", e)))?;

        // Encrypted key format: "v1:" + base64(nonce + ciphertext)
        let encrypted_data = encrypted_key.strip_prefix("v1:")
            .ok_or_else(|| AppError::Encryption("encrypted key must have 'v1:' prefix".into()))?;

        let decoded = BASE64.decode(encrypted_data)
            .map_err(|e| AppError::Encryption(format!("failed to decode encrypted key: {}", e)))?;

        if decoded.len() < 12 {
            return Err(AppError::Encryption("encrypted key too short".into()));
        }

        let (nonce_bytes, ciphertext) = decoded.split_at(12);
        let nonce = Nonce::from_slice(nonce_bytes);

        let plaintext = cipher.decrypt(nonce, ciphertext)
            .map_err(|e| AppError::Encryption(format!("failed to decrypt private key: {}", e)))?;

        String::from_utf8(plaintext)
            .map_err(|e| AppError::Encryption(format!("decrypted key is not valid UTF-8: {}", e)))
    }

    /// Gets the encryption key bytes from configuration.
    /// The key is base64-encoded (not hex).
    fn get_encryption_key_bytes(&self) -> AppResult<Vec<u8>> {
        use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};

        let key = self.config.app.encryption_key.as_ref()
            .ok_or_else(|| AppError::Encryption("encryption key not configured".into()))?;

        let key_bytes = BASE64.decode(key)
            .map_err(|e| AppError::Encryption(format!("invalid encryption key format: {}", e)))?;

        if key_bytes.len() != 32 {
            return Err(AppError::Encryption(
                format!("encryption key must be 32 bytes, got {}", key_bytes.len())
            ));
        }

        Ok(key_bytes)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Note: Full integration tests require a running PostgreSQL database
    // and Kafka instance. Unit tests for individual components are in
    // their respective modules.

    #[test]
    fn test_processor_clone() {
        // Verify that DiplomaProcessor can be cloned (required for use in async closures)
        // This is a compile-time check
    }
}
