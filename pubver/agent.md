# pubver agent notes

## Current verification model

Public verification uses:

1. `EdDSA` verification of the outer QR JWT.
2. `A256GCM` decryption of the `enc` claim.
3. Recomputed `SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt)`.
4. Lookup in `diploma_hashes`.

`diploma_hashes.signature` is not used by the public verification flow.

## JWT contract

Outer JWT payload:

- `sub`
- `diploma_hash`
- `vuz_id`
- `enc`
- `iat`

Decrypted `enc` JSON:

- `full_name`
- `diploma_number`
- `specialty`
- `degree`
- `faculty`
- `year`
- `salt`

`enc` is expected as `base64(nonce|ciphertext|tag)` encrypted with `A256GCM`.

## Database usage

The service reads:

- `universities`
- `diploma_hashes`
- `batch_results`
- `batch_record_attributes`

Important fields:

- `universities.id`
- `universities.name`
- `universities.public_key`
- `universities.vuz_code`
- `diploma_hashes.hash`
- `diploma_hashes.vuz_id`
- `diploma_hashes.diploma_number`
- `diploma_hashes.status`
- `diploma_hashes.revoked_at`
- `batch_results.diploma_hash`
- `batch_results.batch_id`
- `batch_results.record_index`
- `batch_record_attributes.batch_id`
- `batch_record_attributes.record_index`
- `batch_record_attributes.year`
- `batch_record_attributes.specialty`
- `batch_record_attributes.degree`
- `batch_record_attributes.faculty`

`universities.public_key` is expected to contain an `Ed25519` public key.

## Request flow

### `/api/v1/verify`

1. Read `payload`.
2. Decode outer JWT payload without trust.
3. Extract `vuz_id`.
4. Load `universities.public_key`.
5. Verify JWT with `EdDSA` / `Ed25519`.
6. Extract outer claims.
7. Decrypt `enc` with `A256GCM`.
8. Recompute `SHA-256`.
9. Compare with `sub` and `diploma_hash`.
10. Lookup `diploma_hashes.hash`.
11. Return public result.

### `/api/v1/verify/search`

1. Read `vuz_code`.
2. Read `diploma_number`.
3. Query `universities.vuz_code + diploma_hashes.diploma_number`.
4. Join latest matching `batch_results` / `batch_record_attributes`.
5. Return status and public metadata fields.

## Runtime mode

The service always connects to real PostgreSQL. There is no in-memory verification stub mode.
