//! Database models mapping 1-to-1 to DB rows via `sqlx::FromRow`.
//!
//! | Struct | Table | Used for |
//! |---|---|---|
//! | `DiplomaHashRecord` | `diploma_hashes` | Hash + Ed25519 signature store |
//! | `BatchResultRecord` | `batch_results` | Per-diploma encrypted payload + QR JWT |
//! | `BatchRecord` | `batches` | Batch progress tracking |
//! | `UniversityRecord` | `universities` | Fetching the university's private key |

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::FromRow;
use uuid::Uuid;

/// Record in `diploma_hashes` table - stores hash and Ed25519 signature
#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct DiplomaHashRecord {
    /// Unique ID
    pub id: Uuid,
    /// SHA-256 hash of the diploma (64 hex chars)
    pub hash: String,
    /// University ID
    pub vuz_id: Uuid,
    /// Diploma number (e.g., "ДВС-2024-001234")
    pub diploma_number: String,
    /// Base64-encoded Ed25519 signature
    pub signature: String,
    /// Status: active, revoked
    pub status: String,
    /// Creation timestamp
    pub created_at: DateTime<Utc>,
}

/// Record in `batch_results` table - per-diploma processing result
#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct BatchResultRecord {
    /// Unique ID
    pub id: Uuid,
    /// Parent batch ID
    pub batch_id: Uuid,
    /// SHA-256 diploma hash
    pub diploma_hash: String,
    /// Base64 AES-GCM encrypted student payload
    pub encrypted_payload: String,
    /// QR JWT token string
    pub qr_payload: String,
    /// Creation timestamp
    pub created_at: DateTime<Utc>,
}

/// Record in `batches` table - batch progress tracking
#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct BatchRecord {
    /// Unique batch ID
    pub id: Uuid,
    /// University ID
    pub vuz_id: Uuid,
    /// Total records in batch
    pub total_records: u32,
    /// Processed records count
    pub processed_records: u32,
    /// Status: pending, processing, completed, failed
    pub status: String,
    /// Creation timestamp
    pub created_at: DateTime<Utc>,
    /// Completion timestamp (nullable)
    pub completed_at: Option<DateTime<Utc>>,
}

/// Record in `universities` table - university info and keys
#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct UniversityRecord {
    /// Unique university ID
    pub id: Uuid,
    /// University name
    pub name: String,
    /// University short name/code
    pub short_name: String,
    /// Ed25519 private key in PEM format (for signing diplomas)
    pub private_key_pem: String,
    /// Ed25519 public key in PEM format
    pub public_key_pem: String,
    /// Whether the university is active
    pub is_active: bool,
    /// Creation timestamp
    pub created_at: DateTime<Utc>,
}

/// New diploma hash record for insertion
#[derive(Debug, Clone)]
pub struct NewDiplomaHash<'a> {
    pub hash: &'a str,
    pub vuz_id: Uuid,
    pub diploma_number: &'a str,
    pub signature: &'a str,
}

/// New batch result record for insertion
#[derive(Debug, Clone)]
pub struct NewBatchResult<'a> {
    pub batch_id: Uuid,
    pub diploma_hash: &'a str,
    pub encrypted_payload: &'a str,
    pub qr_payload: &'a str,
}
