use sqlx::PgPool;
use uuid::Uuid;

use crate::db::models::{DiplomaHashRecord, NewDiplomaHash, UniversityKeyRecord};
use crate::error::AppResult;

/// Represents the key data retrieved from the database for signing operations.
/// Contains the encrypted private key and associated algorithm information.
#[derive(Debug, Clone)]
pub struct UniversityKeyData {
    pub encrypted_private_key: String,
    pub key_algorithm: String,
    pub encryption_algorithm: String,
}

/// Retrieves the encrypted private key and algorithm information for a given university.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `vuz_id` - UUID of the university
///
/// # Returns
/// * `Ok(UniversityKeyData)` - Contains encrypted key, key algorithm, and encryption algorithm
/// * `Err(sqlx::Error)` - If the query fails or no record is found
///
/// # Example
/// ```rust
/// let key_data = get_university_key(&pool, vuz_id).await?;
/// println!("Key algorithm: {}", key_data.key_algorithm);
/// println!("Encryption algorithm: {}", key_data.encryption_algorithm);
/// ```
pub async fn get_university_key(
    pool: &PgPool,
    vuz_id: Uuid,
) -> AppResult<UniversityKeyData> {
    let record = sqlx::query_as::<_, UniversityKeyRecord>(
        r#"
        SELECT 
            vuz_id,
            encrypted_private_key,
            key_algorithm,
            encryption_algorithm,
            public_key_fingerprint
        FROM university_signing_keys
        WHERE vuz_id = $1
        "#
    )
    .bind(vuz_id)
    .fetch_one(pool)
    .await?;

    Ok(UniversityKeyData {
        encrypted_private_key: record.encrypted_private_key,
        key_algorithm: record.key_algorithm,
        encryption_algorithm: record.encryption_algorithm,
    })
}

/// Inserts a new diploma hash record into the database.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `new_diploma` - The new diploma hash record to insert
///
/// # Returns
/// * `Ok(DiplomaHashRecord)` - The newly created record with database-generated fields
/// * `Err(sqlx::Error)` - If the insert fails
///
/// # Example
/// ```rust
/// let new_diploma = NewDiplomaHash {
///     hash: "abc123...",
///     vuz_id: university_uuid,
///     diploma_number: "ДВС-2024-001234",
///     signature: None,
/// };
/// let record = insert_diploma_hash(&pool, &new_diploma).await?;
/// ```
pub async fn insert_diploma_hash(
    pool: &PgPool,
    new_diploma: &NewDiplomaHash<'_>,
) -> AppResult<DiplomaHashRecord> {
    let record = sqlx::query_as::<_, DiplomaHashRecord>(
        r#"
        INSERT INTO diploma_hashes (hash, vuz_id, diploma_number, status, created_at)
        VALUES ($1, $2, $3, 'active', NOW())
        RETURNING hash, vuz_id, diploma_number, status, revoked_at, created_at
        "#
    )
    .bind(new_diploma.hash)
    .bind(new_diploma.vuz_id)
    .bind(new_diploma.diploma_number)
    .fetch_one(pool)
    .await?;

    Ok(record)
}

/// Checks if all diplomas in a batch have been processed.
/// This function is used to determine when a batch is complete.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `batch_id` - UUID of the batch to check
///
/// # Returns
/// * `Ok(true)` - If the batch is complete
/// * `Ok(false)` - If the batch is not yet complete
/// * `Err(sqlx::Error)` - If the query fails
pub async fn is_batch_complete(
    pool: &PgPool,
    batch_id: Uuid,
) -> AppResult<bool> {
    // This is a placeholder implementation. The actual implementation
    // depends on your batch tracking mechanism.
    // You may need to adjust this based on your actual schema.
    let result: Option<(i64,)> = sqlx::query_as(
        r#"
        SELECT COUNT(*) 
        FROM diploma_hashes dh
        JOIN batch_diplomas bd ON dh.hash = bd.diploma_hash
        WHERE bd.batch_id = $1
        "#
    )
    .bind(batch_id)
    .fetch_optional(pool)
    .await?;

    // If we have a result, check if count > 0
    // This is a simplified check - adjust based on your actual batch completion logic
    Ok(result.map_or(false, |(count,)| count > 0))
}

/// Updates the status of a diploma hash to 'revoked'.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `hash` - The hash of the diploma to revoke
///
/// # Returns
/// * `Ok(DiplomaHashRecord)` - The updated record
/// * `Err(sqlx::Error)` - If the update fails or no record is found
pub async fn revoke_diploma_hash(
    pool: &PgPool,
    hash: &str,
) -> AppResult<DiplomaHashRecord> {
    let record = sqlx::query_as::<_, DiplomaHashRecord>(
        r#"
        UPDATE diploma_hashes
        SET status = 'revoked', revoked_at = NOW()
        WHERE hash = $1
        RETURNING hash, vuz_id, diploma_number, status, revoked_at, created_at
        "#
    )
    .bind(hash)
    .fetch_one(pool)
    .await?;

    Ok(record)
}

/// Retrieves a diploma hash record by its hash value.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `hash` - The hash of the diploma to retrieve
///
/// # Returns
/// * `Ok(DiplomaHashRecord)` - The found record
/// * `Err(sqlx::Error)` - If the query fails or no record is found
pub async fn get_diploma_by_hash(
    pool: &PgPool,
    hash: &str,
) -> AppResult<DiplomaHashRecord> {
    let record = sqlx::query_as::<_, DiplomaHashRecord>(
        r#"
        SELECT hash, vuz_id, diploma_number, status, revoked_at, created_at
        FROM diploma_hashes
        WHERE hash = $1
        "#
    )
    .bind(hash)
    .fetch_one(pool)
    .await?;

    Ok(record)
}

/// Checks if a diploma hash already exists in the database.
///
/// # Arguments
/// * `pool` - PostgreSQL connection pool
/// * `hash` - The hash to check
///
/// # Returns
/// * `Ok(true)` - If the hash exists
/// * `Ok(false)` - If the hash does not exist
/// * `Err(sqlx::Error)` - If the query fails
pub async fn diploma_hash_exists(
    pool: &PgPool,
    hash: &str,
) -> AppResult<bool> {
    let result: Option<(bool,)> = sqlx::query_as(
        r#"
        SELECT TRUE FROM diploma_hashes WHERE hash = $1
        "#
    )
    .bind(hash)
    .fetch_optional(pool)
    .await?;

    Ok(result.is_some())
}

#[cfg(test)]
mod tests {
    use super::*;

    // Note: These tests require a running PostgreSQL database with the schema set up.
    // Use sqlx::testing for integration tests or mock the database for unit tests.
}
