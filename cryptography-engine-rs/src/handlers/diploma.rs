//! Diploma processing handler.
//!
//! Single public function `process` orchestrating the full pipeline:
//!
//! ```text
//! Step  Module called              Action
//! ────  ─────────────────────────  ──────────────────────────────────────────
//!  1.   db::repository             Fetch university record (private key)
//!  2.   cryptography::hashing      generate_salt()
//!  3.   cryptography::hashing      hash_diploma(student, vuz_id, salt)
//!  4.   cryptography::signing      sign_hash(diploma_hash, private_key)
//!  5.   cryptography::encryption   encrypt_payload(student_fields, enc_key)
//!  6.   cryptography::jwt          create_qr_jwt(claims, config)
//!  7.   cryptography::jwt          build_qr_url(token, config)
//!  8.   db::repository             insert_diploma_hash(...)
//!  9.   db::repository             insert_batch_result(...)
//! 10.   db::repository             increment_batch_processed(batch_id)
//! 11.   db::repository             set_batch_completed(batch_id) if last record
//! 12.   kafka::producer            send ProcessingResult { status: Ok }
//! ```
//!
//! On any error at steps 1–11, the handler catches it and publishes
//! `ProcessingResult { status: Error, error: Some(msg) }` so the Gateway
//! can mark the individual record as failed without stalling the rest of the batch.

use std::time::{SystemTime, UNIX_EPOCH};
use sqlx::PgPool;
use tracing::{info, warn, error, debug};

use crate::config::AppConfig;
use crate::cryptography::{
    generate_salt, hash_diploma,
    sign_hash,
    encrypt_payload,
    create_qr_jwt, build_qr_url,
    hashing::StudentFieldsForHash,
    jwt::QrClaims,
};
use crate::db::models::{NewDiplomaHash, NewBatchResult};
use crate::db::repository::{
    get_university, insert_diploma_hash, insert_batch_result,
    increment_batch_processed, set_batch_completed, is_batch_complete,
};
use crate::kafka::messages::{DiplomaTask, ProcessingResult, ProcessingStatus};
use crate::kafka::producer::KafkaProducer;
use crate::error::{AppError, AppResult};

/// Processes a single diploma task through the full cryptographic pipeline.
///
/// This function:
/// 1. Fetches the university's Ed25519 private key from the database
/// 2. Generates a random salt and computes the diploma hash
/// 3. Signs the hash with the university's private key
/// 4. Encrypts the student payload with AES-GCM
/// 5. Creates a QR JWT with the diploma claims
/// 6. Persists results to the database
/// 7. Publishes the processing result to Kafka
///
/// On any error, publishes an error result to Kafka instead of failing.
pub async fn process(
    task: DiplomaTask,
    config: &AppConfig,
    pool: &PgPool,
    producer: &KafkaProducer,
) {
    let batch_id = task.batch_id;
    let vuz_id = task.vuz_id;
    let record_index = task.record_index;
    
    info!(
        batch_id = %batch_id,
        record_index = record_index,
        "Processing diploma task"
    );
    
    // Process the task and handle any errors
    let result = match process_internal(task, config, pool).await {
        Ok((diploma_hash, signature, encrypted_payload, qr_url)) => {
            // Step 10: Increment batch processed counter
            if let Err(e) = increment_batch_processed(pool, batch_id).await {
                warn!(
                    batch_id = %batch_id,
                    error = %e,
                    "Failed to increment batch counter"
                );
            }
            
            // Step 11: Check if batch is complete
            match is_batch_complete(pool, batch_id).await {
                Ok(true) => {
                    if let Err(e) = set_batch_completed(pool, batch_id).await {
                        warn!(
                            batch_id = %batch_id,
                            error = %e,
                            "Failed to mark batch as completed"
                        );
                    }
                    info!(batch_id = %batch_id, "Batch completed");
                }
                Ok(false) => {}
                Err(e) => {
                    warn!(
                        batch_id = %batch_id,
                        error = %e,
                        "Failed to check batch completion status"
                    );
                }
            }
            
            info!(
                batch_id = %batch_id,
                record_index = record_index,
                diploma_hash = %diploma_hash,
                "Diploma processed successfully"
            );
            
            ProcessingResult::success(
                batch_id,
                vuz_id,
                record_index,
                diploma_hash,
                signature,
                encrypted_payload,
                qr_url,
            )
        }
        Err(e) => {
            error!(
                batch_id = %batch_id,
                record_index = record_index,
                error = %e,
                "Failed to process diploma"
            );
            
            ProcessingResult::error(
                batch_id,
                vuz_id,
                record_index,
                e.to_string(),
            )
        }
    };
    
    // Step 12: Send processing result to Kafka
    if let Err(e) = producer.send_result(&result).await {
        error!(
            batch_id = %batch_id,
            record_index = record_index,
            error = %e,
            "Failed to send processing result to Kafka"
        );
    }
}

/// Internal processing function that returns the computed values or an error.
async fn process_internal(
    task: DiplomaTask,
    config: &AppConfig,
    pool: &PgPool,
) -> AppResult<(String, String, String, String)> {
    // Step 1: Fetch university record (private key)
    debug!(vuz_id = %task.vuz_id, "Fetching university record");
    let university = get_university(pool, task.vuz_id).await
        .map_err(|e| AppError::Signing(format!("university not found: {}", e)))?;
    
    // Step 2: Generate random salt
    let salt = generate_salt()?;
    debug!(salt_len = salt.len(), "Generated salt");
    
    // Step 3: Compute diploma hash
    let student_for_hash = StudentFieldsForHash {
        full_name: &task.student.full_name,
        diploma_number: &task.student.diploma_number,
        specialty: &task.student.specialty,
        year: task.student.year,
    };
    let diploma_hash = hash_diploma(&student_for_hash, task.vuz_id, &salt)?;
    debug!(diploma_hash = %diploma_hash, "Computed diploma hash");
    
    // Step 4: Sign the hash
    let signature = sign_hash(&diploma_hash, &university.private_key_pem)?;
    debug!(signature_len = signature.len(), "Signed diploma hash");
    
    // Step 5: Encrypt student payload
    let encryption_key = hex::decode(&config.app.encryption_key)
        .map_err(|e| AppError::Encryption(format!("invalid encryption key: {}", e)))?;
    let encrypted_payload = encrypt_payload(&task.student, &encryption_key)?;
    debug!(payload_len = encrypted_payload.len(), "Encrypted payload");
    
    // Step 6: Create QR JWT
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| AppError::Jwt(jsonwebtoken::errors::Error::from(
            jsonwebtoken::errors::ErrorKind::InvalidToken
        )))?
        .as_secs();
    
    let claims = QrClaims {
        sub: diploma_hash.clone(),
        diploma_hash: diploma_hash.clone(),
        vuz_id: task.vuz_id,
        diploma_number: task.student.diploma_number.clone(),
        student_name: task.student.full_name.clone(),
        specialty: task.student.specialty.clone(),
        year: task.student.year,
        salt: salt.clone(),
        iat: now,
    };
    
    let qr_jwt = create_qr_jwt(&claims, config)?;
    debug!(jwt_len = qr_jwt.len(), "Created QR JWT");
    
    // Step 7: Build QR URL (for reference, stored in qr_payload)
    let qr_url = build_qr_url(&qr_jwt, config);
    
    // Step 8: Insert diploma hash record
    let hash_record = NewDiplomaHash {
        hash: &diploma_hash,
        vuz_id: task.vuz_id,
        diploma_number: &task.student.diploma_number,
        signature: &signature,
    };
    insert_diploma_hash(pool, &hash_record).await?;
    debug!("Inserted diploma hash record");
    
    // Step 9: Insert batch result record
    let batch_record = NewBatchResult {
        batch_id: task.batch_id,
        diploma_hash: &diploma_hash,
        encrypted_payload: &encrypted_payload,
        qr_payload: &qr_url,
    };
    insert_batch_result(pool, &batch_record).await?;
    debug!("Inserted batch result record");
    
    Ok((diploma_hash, signature, encrypted_payload, qr_url))
}
