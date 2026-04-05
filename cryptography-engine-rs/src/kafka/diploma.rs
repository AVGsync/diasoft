use std::sync::Arc;

use sqlx::PgPool;
use sqlx::Row;
use tracing::{debug, info, warn, error};

use crate::config::AppConfig;
use crate::cryptography::{
    create_qr_jwt, derive_salt, hash_diploma,
    sign_hash, StudentFieldsForHash, build_qr_claims,
};
use crate::db::models::NewDiplomaHash;
use crate::db::repository::{get_diploma_by_vuz_and_number, get_university_key, insert_diploma_hash};
use crate::error::{AppError, AppResult};
use crate::kafka::messages::{DiplomaTask, ProcessingResult, ProcessingStatus};
use crate::kafka::producer::KafkaProducer;

#[derive(Clone)]
pub struct DiplomaProcessor {
    config: Arc<AppConfig>,
    pool: Arc<PgPool>,
    producer: Arc<KafkaProducer>,
}

impl DiplomaProcessor {
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

    pub async fn process(&self, task: DiplomaTask) -> AppResult<()> {
        info!(
            batch_id = %task.batch_id,
            record_index = task.record_index,
            vuz_id = %task.vuz_id,
            "Processing diploma task"
        );

        let result = match self.process_inner(&task).await {
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
                result
            }
            Err(e) => {
                error!(
                    batch_id = %task.batch_id,
                    record_index = task.record_index,
                    error = %e,
                    "Unexpected error during diploma processing"
                );
                
                let error_result = ProcessingResult::error(
                    task.batch_id,
                    task.vuz_id,
                    task.record_index,
                    e.to_string(),
                );

                error_result
            }
        };

        self.persist_processing_result(&result).await?;

        if let Err(e) = self.producer.send_result(&result).await {
            warn!(
                batch_id = %task.batch_id,
                record_index = task.record_index,
                error = %e,
                "Failed to send processing result to Kafka, but database state is already updated"
            );
        }

        Ok(())
    }

    async fn process_inner(&self, task: &DiplomaTask) -> AppResult<ProcessingResult> {
        let key_data = match get_university_key(&self.pool, task.vuz_id).await {
            Ok(key_data) => key_data,
            Err(AppError::Db(sqlx::Error::RowNotFound)) => {
                return Err(AppError::Signing(
                    "university signing key is not configured".to_string(),
                ))
            }
            Err(err) => return Err(err),
        };
        debug!(
            vuz_id = %task.vuz_id,
            key_algorithm = %key_data.key_algorithm,
            "Retrieved university signing key"
        );

        let salt = derive_salt(task.vuz_id, &task.student.diploma_number)?;
        let student_for_hash = StudentFieldsForHash {
            full_name: &task.student.full_name,
            diploma_number: &task.student.diploma_number,
            specialty: &task.student.specialty,
            degree: &task.student.degree,
            faculty: &task.student.faculty,
            year: task.student.year,
        };
        let diploma_hash = hash_diploma(&student_for_hash, task.vuz_id, &salt)?;
        debug!(diploma_hash = %diploma_hash, "Generated diploma hash");

        let existing_diploma = get_diploma_by_vuz_and_number(
            &self.pool,
            task.vuz_id,
            &task.student.diploma_number,
        ).await?;

        if let Some(existing_diploma) = &existing_diploma {
            if existing_diploma.hash != diploma_hash {
                return Err(AppError::Hashing(
                    "diploma number already exists with another hash".to_string(),
                ));
            }

            debug!(
                diploma_hash = %diploma_hash,
                diploma_number = %task.student.diploma_number,
                "Diploma already exists with the same hash, reusing deterministic identity"
            );
        }

        let private_key_pem = self.decrypt_private_key(&key_data.encrypted_private_key)?;

        let signature = sign_hash(&diploma_hash, &private_key_pem)?;
        debug!(signature_len = signature.len(), "Signed diploma hash");

        let encryption_key_bytes = self.get_encryption_key_bytes()?;
        debug!(key_len = encryption_key_bytes.len(), "Retrieved encryption key");

        let qr_claims = build_qr_claims(
            diploma_hash.clone(),
            task.vuz_id,
            task.student.diploma_number.clone(),
            task.student.full_name.clone(),
            task.student.specialty.clone(),
            task.student.degree.clone(),
            task.student.faculty.clone(),
            task.student.year,
            salt.clone(),
            &encryption_key_bytes,
        )?;

        let qr_token = create_qr_jwt(&qr_claims, &private_key_pem)?;
        
        let qr_payload = qr_token;
        debug!(qr_payload_len = qr_payload.len(), "Generated QR payload");

        let new_diploma = NewDiplomaHash {
            hash: &diploma_hash,
            vuz_id: task.vuz_id,
            diploma_number: &task.student.diploma_number,
            signature: Some(&signature),
        };
        if existing_diploma.is_none() {
            insert_diploma_hash(&self.pool, &new_diploma).await?;
            debug!(diploma_hash = %diploma_hash, "Persisted diploma hash to database");
        }

        Ok(ProcessingResult::success(
            task.batch_id,
            task.vuz_id,
            task.record_index,
            diploma_hash,
            qr_payload,
        ))
    }

    fn decrypt_private_key(&self, encrypted_key: &str) -> AppResult<String> {
        use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
        use aes_gcm::{
            aead::Aead,
            Aes256Gcm, KeyInit, Nonce,
        };

        let master_key = self.config.app.encryption_key.trim();
        if master_key.is_empty() {
            return Err(AppError::Encryption("master encryption key not configured".into()));
        }

        let key_bytes = BASE64.decode(master_key)
            .map_err(|e| AppError::Encryption(format!("invalid master key format: {}", e)))?;

        if key_bytes.len() != 32 {
            return Err(AppError::Encryption(
                format!("master key must be 32 bytes, got {}", key_bytes.len())
            ));
        }

        let cipher = Aes256Gcm::new_from_slice(&key_bytes)
            .map_err(|e| AppError::Encryption(format!("failed to initialize cipher: {}", e)))?;

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

    fn get_encryption_key_bytes(&self) -> AppResult<Vec<u8>> {
        use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};

        let key_bytes = BASE64.decode(&self.config.jwt.payload_secret)
            .map_err(|e| AppError::Encryption(format!("invalid payload secret format: {}", e)))?;

        if key_bytes.len() != 32 {
            return Err(AppError::Encryption(
                format!("payload secret must be 32 bytes, got {}", key_bytes.len())
            ));
        }

        Ok(key_bytes)
    }

    async fn persist_processing_result(&self, result: &ProcessingResult) -> AppResult<()> {
        let mut tx = self.pool.begin().await?;

        let existing_row = sqlx::query(
            r#"
            SELECT status
            FROM batch_results
            WHERE batch_id = $1 AND record_index = $2
            FOR UPDATE
            "#,
        )
        .bind(result.batch_id)
        .bind(result.record_index as i32)
        .fetch_optional(&mut *tx)
        .await?;

        if existing_row.is_some() {
            debug!(
                batch_id = %result.batch_id,
                record_index = result.record_index,
                "Batch result already persisted, skipping direct database write"
            );
            tx.commit().await?;
            return Ok(());
        }

        match &result.status {
            ProcessingStatus::Ok => {
                sqlx::query(
                    r#"
                    INSERT INTO batch_results (batch_id, record_index, diploma_hash, qr_payload, status, error)
                    VALUES ($1, $2, $3, $4, 'ok', NULL)
                    "#,
                )
                .bind(result.batch_id)
                .bind(result.record_index as i32)
                .bind(&result.diploma_hash)
                .bind(result.qr_payload.as_deref())
                .execute(&mut *tx)
                .await?;
            }
            ProcessingStatus::Error => {
                sqlx::query(
                    r#"
                    INSERT INTO batch_results (batch_id, record_index, diploma_hash, qr_payload, status, error)
                    VALUES ($1, $2, NULL, NULL, 'error', $3)
                    "#,
                )
                .bind(result.batch_id)
                .bind(result.record_index as i32)
                .bind(result.error.as_deref().unwrap_or("processing error"))
                .execute(&mut *tx)
                .await?;
            }
        }

        sqlx::query(
            r#"
            UPDATE batches
            SET processed_records = processed_records + 1
            WHERE id = $1
            "#,
        )
        .bind(result.batch_id)
        .execute(&mut *tx)
        .await?;

        let batch_row = sqlx::query(
            r#"
            SELECT
                total_records,
                processed_records,
                (
                    SELECT COUNT(*)
                    FROM batch_results
                    WHERE batch_id = $1 AND status = 'error'
                ) AS error_count
            FROM batches
            WHERE id = $1
            "#,
        )
        .bind(result.batch_id)
        .fetch_one(&mut *tx)
        .await?;

        let total_records: i32 = batch_row.try_get("total_records")?;
        let processed_records: i32 = batch_row.try_get("processed_records")?;
        let error_count: i64 = batch_row.try_get("error_count")?;

        if processed_records >= total_records {
            let final_status = if error_count > 0 { "failed" } else { "completed" };

            sqlx::query(
                r#"
                UPDATE batches
                SET status = $2, completed_at = COALESCE(completed_at, NOW())
                WHERE id = $1
                "#,
            )
            .bind(result.batch_id)
            .bind(final_status)
            .execute(&mut *tx)
            .await?;
        }

        tx.commit().await?;

        info!(
            batch_id = %result.batch_id,
            record_index = result.record_index,
            status = ?result.status,
            "Persisted processing result directly to database"
        );

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn test_processor_clone() {
    }
}
