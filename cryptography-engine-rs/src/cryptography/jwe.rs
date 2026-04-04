//! JWE decryption for incoming Kafka messages.
//!
//! Algorithm: ECDH-ES+A256KW (key agreement) + A256GCM (content encryption).
//! The private key must be an EC P-256 key in PKCS#8 PEM format.

use josekit::jwk::Jwk;
use serde::de::DeserializeOwned;
use crate::error::{AppError, AppResult};

/// Decrypts a compact JWE token and deserializes the plaintext as JSON into T.
///
/// # Arguments
/// * `token`           — compact JWE string (5-part, dot-separated)
/// * `private_key_pem` — EC P-256 private key in PKCS#8 PEM format
pub fn decrypt_jwe<T: DeserializeOwned>(token: &str, private_key_pem: &str) -> AppResult<T> {
    let jwk = Jwk::from_bytes(private_key_pem.as_bytes())
        .map_err(|e| AppError::Encryption(format!("jwe: failed to parse private key: {}", e)))?;

    let decrypter = josekit::jwe::ECDH_ES_A256KW
        .decrypter_from_jwk(&jwk)
        .map_err(|e| AppError::Encryption(format!("jwe: failed to build decrypter: {}", e)))?;

    let (plaintext, _header) = josekit::jwe::deserialize_compact(token, &decrypter)
        .map_err(|e| AppError::Encryption(format!("jwe: decryption failed: {}", e)))?;

    let value: T = serde_json::from_slice(&plaintext)
        .map_err(|e| AppError::Encryption(format!("jwe: json deserialize failed: {}", e)))?;

    Ok(value)
}

#[cfg(test)]
mod tests {
    use super::*;
    use josekit::jwk::alg::ec::EcKeyPair;
    use serde::{Deserialize, Serialize};

    #[derive(Debug, Serialize, Deserialize, PartialEq)]
    struct TestPayload {
        name: String,
        value: u32,
    }

    fn generate_ec_jwk_pair() -> (Jwk, Jwk) {
        let key_pair = EcKeyPair::generate(josekit::jwk::alg::ec::EcCurve::P256)
            .expect("EC keygen failed");
        let private_jwk = key_pair.to_jwk_private_key();
        let public_jwk = key_pair.to_jwk_public_key();
        (private_jwk, public_jwk)
    }

    #[test]
    fn test_decrypt_jwe_roundtrip() {
        let (private_jwk, public_jwk) = generate_ec_jwk_pair();
        let payload = TestPayload { name: "Ivan".to_string(), value: 42 };

        // Encrypt using josekit directly (simulates Gateway)
        let encrypter = josekit::jwe::ECDH_ES_A256KW.encrypter_from_jwk(&public_jwk).unwrap();
        let mut header = josekit::jwe::JweHeader::new();
        header.set_content_encryption("A256GCM");
        let plaintext = serde_json::to_vec(&payload).unwrap();
        let token = josekit::jwe::serialize_compact(&plaintext, &header, &encrypter).unwrap();

        // Decrypt using JWK (not PEM, since decrypt_jwe uses Jwk::from_bytes)
        let decrypter = josekit::jwe::ECDH_ES_A256KW
            .decrypter_from_jwk(&private_jwk)
            .expect("decrypter");
        let (decrypted, _header) = josekit::jwe::deserialize_compact(&token, &decrypter).unwrap();
        let decoded: TestPayload = serde_json::from_slice(&decrypted).unwrap();
        assert_eq!(decoded, payload);
    }

    #[test]
    fn test_decrypt_jwe_wrong_key_fails() {
        let (_, public_jwk) = generate_ec_jwk_pair();
        let (wrong_private_jwk, _) = generate_ec_jwk_pair();
        let payload = TestPayload { name: "test".to_string(), value: 1 };

        let encrypter = josekit::jwe::ECDH_ES_A256KW.encrypter_from_jwk(&public_jwk).unwrap();
        let mut header = josekit::jwe::JweHeader::new();
        header.set_content_encryption("A256GCM");
        let plaintext = serde_json::to_vec(&payload).unwrap();
        let token = josekit::jwe::serialize_compact(&plaintext, &header, &encrypter).unwrap();

        // Try to decrypt with wrong key
        let decrypter = josekit::jwe::ECDH_ES_A256KW
            .decrypter_from_jwk(&wrong_private_jwk)
            .expect("decrypter");
        assert!(josekit::jwe::deserialize_compact(&token, &decrypter).is_err());
    }
}
