//! Database repository functions for all DB writes and reads.
//!
//! All DB operations performed by this service:
//! - `insert_diploma_hash` - INSERT into `diploma_hashes`
//! - `insert_batch_result` - INSERT into `batch_results`
//! - `increment_batch_processed` - UPDATE `batches` counter
//! - `set_batch_completed` - UPDATE `batches` status
//! - `get_university` - SELECT university for Ed25519 private key

use sqlx::PgPool;
use uuid::Uuid;

use crate::db::models::{DiplomaHashRecord, UniversityRecord, BatchRecord, NewDiplomaHash, NewBatchResult};
use crate::error::{AppError, AppResult};

/// Inserts a new diploma hash record.
///
/// Uses `ON CONFLICT DO NOTHING` for idempotent retries - if the hash
/// already exists, the operation succeeds without modification.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `record` - New diploma hash data
pub async fn insert_diploma_hash(
    pool: &PgPool,
    record: &NewDiplomaHash<'_>,
) -> AppResult<()> {
    sqlx::query!(
        r#"
        INSERT INTO diploma_hashes (hash, vuz_id, diploma_number, signature, status, created_at)
        VALUES ($1, $2, $3, $4, 'active', NOW())
        ON CONFLICT (hash) DO NOTHING
        "#,
        record.hash,
        record.vuz_id,
        record.diploma_number,
        record.signature,
    )
    .execute(pool)
    .await?;
    
    Ok(())
}

/// Gets a diploma hash record by hash value.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `hash` - SHA-256 diploma hash
pub async fn get_diploma_by_hash(
    pool: &PgPool,
    hash: &str,
) -> AppResult<Option<DiplomaHashRecord>> {
    let record = sqlx::query_as!(
        DiplomaHashRecord,
        r#"
        SELECT id, hash, vuz_id, diploma_number, signature, status, created_at
        FROM diploma_hashes
        WHERE hash = $1
        "#,
        hash
    )
    .fetch_optional(pool)
    .await?;
    
    Ok(record)
}

/// Inserts a new batch result record.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `record` - New batch result data
pub async fn insert_batch_result(
    pool: &PgPool,
    record: &NewBatchResult<'_>,
) -> AppResult<()> {
    sqlx::query!(
        r#"
        INSERT INTO batch_results (batch_id, diploma_hash, encrypted_payload, qr_payload, created_at)
        VALUES ($1, $2, $3, $4, NOW())
        "#,
        record.batch_id,
        record.diploma_hash,
        record.encrypted_payload,
        record.qr_payload,
    )
    .execute(pool)
    .await?;
    
    Ok(())
}

/// Increments the processed records counter for a batch.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `batch_id` - Batch UUID
pub async fn increment_batch_processed(
    pool: &PgPool,
    batch_id: Uuid,
) -> AppResult<()> {
    sqlx::query!(
        r#"
        UPDATE batches
        SET processed_records = processed_records + 1
        WHERE id = $1
        "#,
        batch_id
    )
    .execute(pool)
    .await?;
    
    Ok(())
}

/// Marks a batch as completed.
///
/// Sets status to 'completed' and records the completion timestamp.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `batch_id` - Batch UUID
pub async fn set_batch_completed(
    pool: &PgPool,
    batch_id: Uuid,
) -> AppResult<()> {
    sqlx::query!(
        r#"
        UPDATE batches
        SET status = 'completed', completed_at = NOW()
        WHERE id = $1
        "#,
        batch_id
    )
    .execute(pool)
    .await?;
    
    Ok(())
}

/// Gets a batch record by ID.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `batch_id` - Batch UUID
pub async fn get_batch(
    pool: &PgPool,
    batch_id: Uuid,
) -> AppResult<Option<BatchRecord>> {
    let record = sqlx::query_as!(
        BatchRecord,
        r#"
        SELECT id, vuz_id, total_records, processed_records, status, created_at, completed_at
        FROM batches
        WHERE id = $1
        "#,
        batch_id
    )
    .fetch_optional(pool)
    .await?;
    
    Ok(record)
}

/// Gets a university record by ID.
///
/// Used to retrieve the Ed25519 private key before signing a diploma.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `vuz_id` - University UUID
pub async fn get_university(
    pool: &PgPool,
    vuz_id: Uuid,
) -> AppResult<UniversityRecord> {
    let record = sqlx::query_as!(
        UniversityRecord,
        r#"
        SELECT id, name, short_name, private_key_pem, public_key_pem, is_active, created_at
        FROM universities
        WHERE id = $1
        "#,
        vuz_id
    )
    .fetch_optional(pool)
    .await?;
    
    record
        .ok_or_else(|| AppError::Db(sqlx::Error::RowNotFound))
        .map_err(|e| AppError::Signing(format!("university not found: {}", e)))
}

/// Checks if all records in a batch have been processed.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `batch_id` - Batch UUID
///
/// # Returns
/// `true` if processed_records >= total_records
pub async fn is_batch_complete(
    pool: &PgPool,
    batch_id: Uuid,
) -> AppResult<bool> {
    let batch = get_batch(pool, batch_id).await?;
    
    Ok(batch
        .map(|b| b.processed_records >= b.total_records)
        .unwrap_or(false))
}
