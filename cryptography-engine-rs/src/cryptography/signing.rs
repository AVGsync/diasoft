use ed25519_dalek::{Signature, Signer, SigningKey, VerifyingKey};
use pkcs8::DecodePrivateKey;
use crate::error::{AppError, AppResult};

pub fn sign<T: AsRef<[u8]>>(data: T, private_key_pem: &str) -> AppResult<String> {
    let signing_key = SigningKey::from_pkcs8_pem(private_key_pem)
        .map_err(|e| AppError::Signing(format!("failed to parse private key: {}", e)))?;
    let signature = signing_key.sign(data.as_ref());
    Ok(hex::encode(signature.to_bytes()))
}

pub fn sign_hash(hash: &str, private_key_pem: &str) -> AppResult<String> {
    sign(hash.as_bytes(), private_key_pem)
}

pub fn verify_signature<T: AsRef<[u8]>>(
    data: T,
    signature_hex: &str,
    public_key_bytes: &[u8],
) -> AppResult<bool> {
    let signature_bytes = hex::decode(signature_hex)
        .map_err(|e| AppError::Signing(format!("failed to decode signature hex: {}", e)))?;
    let signature = Signature::from_slice(&signature_bytes)
        .map_err(|e| AppError::Signing(format!("invalid signature bytes: {}", e)))?;
    let verifying_key = VerifyingKey::from_bytes(
        public_key_bytes.try_into()
            .map_err(|_| AppError::Signing("invalid public key length (expected 32 bytes)".to_string()))?
    ).map_err(|e| AppError::Signing(format!("invalid public key: {}", e)))?;
    match verifying_key.verify_strict(data.as_ref(), &signature) {
        Ok(()) => Ok(true),
        Err(_) => Ok(false),
    }
}
