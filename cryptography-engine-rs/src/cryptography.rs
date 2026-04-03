//! Cryptographic primitives for diploma processing.
//!
//! Pure, side-effect-free cryptographic operations.
//! No DB or Kafka imports — only `crate::error::AppError` crosses the module boundary.

pub mod hashing;
pub mod signing;
pub mod encryption;
pub mod jwt;

// Re-export the most-used symbols to flatten call-site imports
pub use hashing::{generate_salt, hash_diploma};
pub use signing::{sign_hash, verify_signature};
pub use encryption::{encrypt_payload, decrypt_payload};
pub use jwt::{create_qr_jwt, build_qr_url, create_share_link_jwt};
