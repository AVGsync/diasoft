//! Kafka message types for both topics.
//!
//! Serde structs for both Kafka topics. No internal imports beyond
//! `serde`, `uuid`, and `chrono`.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Inbound message - consumed from `diplomas.raw_tasks` (published by Gateway)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiplomaTask {
    /// Batch ID this task belongs to
    pub batch_id: Uuid,
    /// University ID
    pub vuz_id: Uuid,
    /// Index of this record within the batch (0-based)
    pub record_index: u32,
    /// Total records in this batch
    pub total_in_batch: u32,
    /// Student data fields
    pub student: StudentFields,
    /// Task creation timestamp
    pub created_at: DateTime<Utc>,
}

/// Student data fields from CSV upload
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StudentFields {
    /// Full name (ФИО)
    pub full_name: String,
    /// Diploma number (e.g., "ДВС-2024-001234")
    pub diploma_number: String,
    /// Specialty code and name
    pub specialty: String,
    /// Degree type: "Бакалавр" | "Магистр" | "Специалист"
    pub degree: String,
    /// Graduation year
    pub year: u16,
    /// Faculty name
    pub faculty: String,
    /// Original CSV row for audit/debugging
    #[serde(default)]
    pub raw_csv_row: Option<String>,
}

/// Outbound message - published to `diplomas.processing_results` (consumed by Gateway)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProcessingResult {
    /// Batch ID this result belongs to
    pub batch_id: Uuid,
    /// University ID
    pub vuz_id: Uuid,
    /// Index of this record within the batch
    pub record_index: u32,
    /// SHA-256 diploma hash (64 hex chars)
    pub diploma_hash: String,
    /// Base64-encoded Ed25519 signature
    pub signature: String,
    /// Base64-encoded AES-GCM encrypted student payload
    pub encrypted_payload: String,
    /// Ready-to-use QR JWT string
    pub qr_payload: String,
    /// Processing status
    pub status: ProcessingStatus,
    /// Error message (if status is Error)
    pub error: Option<String>,
    /// Processing completion timestamp
    pub processed_at: DateTime<Utc>,
}

/// Processing status for a diploma task
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ProcessingStatus {
    /// Successfully processed
    Ok,
    /// Processing failed
    Error,
}

impl ProcessingResult {
    /// Creates a successful processing result
    pub fn success(
        batch_id: Uuid,
        vuz_id: Uuid,
        record_index: u32,
        diploma_hash: String,
        signature: String,
        encrypted_payload: String,
        qr_payload: String,
    ) -> Self {
        Self {
            batch_id,
            vuz_id,
            record_index,
            diploma_hash,
            signature,
            encrypted_payload,
            qr_payload,
            status: ProcessingStatus::Ok,
            error: None,
            processed_at: Utc::now(),
        }
    }
    
    /// Creates a failed processing result
    pub fn error(
        batch_id: Uuid,
        vuz_id: Uuid,
        record_index: u32,
        error_message: String,
    ) -> Self {
        Self {
            batch_id,
            vuz_id,
            record_index,
            diploma_hash: String::new(),
            signature: String::new(),
            encrypted_payload: String::new(),
            qr_payload: String::new(),
            status: ProcessingStatus::Error,
            error: Some(error_message),
            processed_at: Utc::now(),
        }
    }
}
