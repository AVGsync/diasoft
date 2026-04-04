use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiplomaTask {
    pub batch_id: Uuid,
    pub vuz_id: Uuid,
    pub record_index: u32,
    pub total_in_batch: u32,
    pub student: StudentFields,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StudentFields {
    pub full_name: String,
    pub diploma_number: String,
    pub specialty: String,
    pub degree: String,
    pub year: u16,
    pub faculty: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProcessingResult {
    pub batch_id: Uuid,
    pub vuz_id: Uuid,
    pub record_index: u32,
    pub diploma_hash: String,
    pub qr_payload: Option<String>,
    pub status: ProcessingStatus,
    pub error: Option<String>,
    pub processed_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ProcessingStatus {
    Ok,
    Error,
}

impl ProcessingResult {
    pub fn success(
        batch_id: Uuid,
        vuz_id: Uuid,
        record_index: u32,
        diploma_hash: String,
        qr_payload: String,
    ) -> Self {
        Self {
            batch_id,
            vuz_id,
            record_index,
            diploma_hash,
            qr_payload: Some(qr_payload),
            status: ProcessingStatus::Ok,
            error: None,
            processed_at: Utc::now(),
        }
    }
    
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
            qr_payload: None,
            status: ProcessingStatus::Error,
            error: Some(error_message),
            processed_at: Utc::now(),
        }
    }
}
