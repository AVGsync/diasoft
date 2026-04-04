use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::FromRow;
use uuid::Uuid;

#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct DiplomaHashRecord {
    pub hash: String,
    pub vuz_id: Uuid,
    pub diploma_number: String,
    #[serde(default)]
    pub status: String,
    pub revoked_at: Option<DateTime<Utc>>,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, FromRow, Serialize, Deserialize)]
pub struct UniversityKeyRecord {
    pub vuz_id: Uuid,
    pub encrypted_private_key: String,
    pub key_algorithm: String,
    pub encryption_algorithm: String,
    pub public_key_fingerprint: String,
}

#[derive(Debug, Clone)]
pub struct NewDiplomaHash<'a> {
    pub hash: &'a str,
    pub vuz_id: Uuid,
    pub diploma_number: &'a str,
    pub signature: Option<&'a str>,
}

#[derive(Debug, Clone)]
pub struct NewBatchResult<'a> {
    pub batch_id: Uuid,
    pub diploma_hash: &'a str,
    pub encrypted_payload: Option<&'a str>,
    pub qr_payload: &'a str,
}
