pub mod hashing;
pub mod signing;
pub mod encryption;
pub mod jwt;
pub mod jwe;

pub use hashing::{generate_salt, hash_diploma, StudentFieldsForHash};
pub use signing::{sign_hash, verify_signature};
pub use encryption::{encrypt_payload, decrypt_payload};
pub use jwt::{
    create_qr_jwt, build_qr_url, verify_qr_jwt,
    QrClaims, EncryptedStudentData, build_qr_claims, decrypt_qr_claims,
};
pub use jwe::decrypt_jwe;
