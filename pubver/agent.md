# pubver agent notes

## Current verification model

Public verification uses only:

1. `RS256` verification of the QR JWT.
2. Recomputed `SHA-256(full_name|diploma_number|specialty|year|vuz_id|salt)`.
3. Lookup in `diploma_hashes`.

`diploma_hashes.signature` is not used by the public verification flow.

## Database usage

The service currently reads only:

- `universities`
- `diploma_hashes`

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

## Temporary placeholders

`year` and `specialty` are intentionally kept in:

- domain models;
- JSON responses;
- OpenAPI schema.

For now they are not stored in PostgreSQL and are returned as `null` placeholders.

`vuz_code` is the public university code in a format like `001X7276`, i.e. a registry code rather than a mnemonic alias.

## Request flow

### `/api/v1/verify`

1. Read `payload`.
2. Decode payload without trust.
3. Extract `vuz_id`.
4. Load `universities.public_key`.
5. Verify JWT with `RS256`.
6. Extract full claims.
7. Recompute SHA-256.
8. Compare with `sub` and `diploma_hash`.
9. Lookup `diploma_hashes.hash`.
10. Return public result.

### `/api/v1/verify/search`

1. Read `vuz_code`.
2. Read `diploma_number`.
3. Query `universities.vuz_code + diploma_hashes.diploma_number`.
4. Return status and placeholder metadata fields.
