# Crypto Engine (cryptography-engine-rs)

> Current contract: `diploma_hash = SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt)` and QR JWT uses `EdDSA` with encrypted `enc` payload.

A Stateless cryptographic processing worker for the diploma verification system.

## Overview

The Crypto Engine is a Rust-based microservice that performs all cryptographic operations for diploma verification. It consumes raw diploma tasks from Kafka, processes them (hashing, signing, encrypting, JWT generation), persists results to PostgreSQL, and publishes processed records back to Kafka.

    
**Key characteristic:** This service never exposes an HTTP API — all I/O is through Kafka + PostgreSQL.

## Data Flow

```
Gateway
  └─► Kafka: diplomas.raw_tasks
            │
            ▼
      [Crypto Engine]
        1. Fetch university Ed25519 private key from DB (universities table)
        2. Generate random 32-byte salt
        3. SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt) → diploma_hash
        4. Ed25519.sign(diploma_hash, vuz_private_key) → signature
        5. Encrypt student payload (AES-GCM) → encrypted_payload
        6. EdDSA JWT { diploma_hash, salt, student fields, exp: null } → qr_payload
        7. INSERT diploma_hashes (hash, vuz_id, diploma_number, signature)
        8. INSERT batch_results (batch_id, diploma_hash, encrypted_payload, qr_payload)
        9. UPDATE batches counter / status
            │
            ▼
      Kafka: diplomas.processing_results → Gateway
```

## Project Structure

```
cryptography-engine-rs/
├── Cargo.toml
├── config/
│   └── default.toml (optional)
├── src/
│   ├── main.rs              # Entry point
│   ├── lib.rs               # Module declarations
│   ├── config.rs            # Configuration structs
│   ├── error.rs             # Error types
│   ├── cryptography.rs      # Cryptography module root
│   ├── cryptography/
│   │   ├── hashing.rs       # Salt generation and diploma hashing
│   │   ├── signing.rs       # Ed25519 digital signatures
│   │   ├── encryption.rs    # AES-GCM payload encryption
│   │   └── jwt.rs            # JWT generation (QR and share-link)
│   ├── db.rs                # Database module root
│   ├── db/
│   │   ├── models.rs        # Database models (structs)
│   │   └── repository.rs    # Database operations
│   ├── kafka.rs             # Kafka module root
│   ├── kafka/
│   │   ├── messages.rs      # Kafka message types
│   │   ├── consumer.rs       # Kafka consumer wrapper
│   │   └── producer.rs       # Kafka producer wrapper
│   ├── handlers.rs           # Handlers module root
│   └── handlers/
│       └── diploma.rs        # Diploma processing handler
└── README.md
```

## File Responsibilities

### [`src/main.rs`](src/main.rs)
**Entry point** - Boots the service and orchestrates all components.

**Responsibilities:**
- Initializes `tracing` subscriber (log level from `RUST_LOG` env var)
- Loads `AppConfig` via `dotenvy` + `config` crate
- Creates `PgPool` (sqlx) and `KafkaConsumer` / `KafkaProducer`
- Spawns the consumer loop, passing each `DiplomaTask` to `handlers::diploma::process`
- Handles graceful shutdown on SIGTERM / SIGINT

---

### [`src/lib.rs`](src/lib.rs)
**Module declarations** - Declares all public modules for the binary and integration tests.

**Exports:**
- `config` - Application configuration
- `error` - Error types
- `cryptography` - Cryptographic primitives
- `db` - Database interaction
- `kafka` - Kafka consumer/producer
- `handlers` - Business logic handlers

---

### [`src/config.rs`](src/config.rs)
**Configuration management** - Strongly-typed application configuration.

**Structs:**
| Struct | Purpose |
|-------|---------|
| `AppConfig` | Top-level configuration container |
| `KafkaConfig` | Kafka brokers, topics, consumer group |
| `DatabaseConfig` | PostgreSQL connection URL |
| `JwtConfig` | Ed25519 private key (QR JWTs), HMAC secret (share-links) |
| `AppSettings` | Verification base URL, encryption key |

**Configuration sources:**
- `config/default.toml` (optional base config)
- `APP__<section>__<key>` environment variables

---

### [`src/error.rs`](src/error.rs)
**Central error handling** - Application-wide error enum.

**Error variants:**
| Variant | Source |
|---------|--------|
| `Hashing` | SHA-256 or salt generation failures |
| `Signing` | Ed25519 key parsing or signature errors |
| `Encryption` | AES-GCM cipher failures |
| `Jwt` | JWT encode/decode (jsonwebtoken) |
| `Db` | PostgreSQL errors (sqlx) |
| `Kafka` | Kafka errors (rdkafka) |
| `Serde` | JSON serialization errors |
| `Config` | Configuration loading errors |
| `Io` | Standard IO errors |

---

### [`src/cryptography.rs`](src/cryptography.rs)
**Cryptography module root** - Re-exports commonly used functions.

**Re-exports:**
- `generate_salt`, `hash_diploma` from hashing
- `sign_hash`, `verify_signature` from signing
- `encrypt_payload`, `decrypt_payload` from encryption
- `create_qr_jwt`, `build_qr_url`, `create_share_link_jwt` from jwt

---

### [`src/cryptography/hashing.rs`](src/cryptography/hashing.rs)
**Diploma hashing** - Canonical hash computation.

**Functions:**
| Function | Description |
|----------|-------------|
| `generate_salt()` | Generates cryptographically random 32-byte hex salt (64 chars) |
| `hash_diploma(student, vuz_id, salt)` | Computes SHA-256 hash of canonical string |

**Hash format:**
```
raw = "{full_name}|{diploma_number}|{specialty}|{year}|{vuz_id}|{salt}"
hash = SHA-256(raw) → 64-char hex string
```

**Why salt in JWT?** The salt is stored in the QR JWT so the verifier service can independently recompute the hash without DB lookups.

---

### [`src/cryptography/signing.rs`](src/cryptography/signing.rs)
**Digital signatures** - Ed25519 signing operations.

**Functions:**
| Function | Description |
|----------|-------------|
| `sign_hash(hash, private_key_pem)` | Signs hash with university's Ed25519 key → base64 signature |
| `verify_signature(hash, signature, public_key_pem)` | Verifies signature (used by Verifier service) |

**Key management:** University private keys are fetched from `universities` table at processing time, not stored in config.

---

### [`src/cryptography/encryption.rs`](src/cryptography/encryption.rs)
**Payload encryption** - AES-256-GCM encryption for student data.

**Functions:**
| Function | Description |
|----------|-------------|
| `encrypt_payload(data, key)` | Encrypts JSON-serializable data → base64(nonce + ciphertext) |
| `decrypt_payload(ciphertext, key)` | Decrypts base64 ciphertext → deserialized data |

**Format:** `[12-byte nonce][ciphertext]` encoded as base64

---

### [`src/cryptography/jwt.rs`](src/cryptography/jwt.rs)
**JWT generation** - Two token types for different purposes.

**Token types:**
| Type | Algorithm | Expiration | Purpose |
|------|-----------|------------|---------|
| QR JWT | EdDSA | None (eternal) | Embedded in QR codes, revocation via DB |
| Share-link JWT | HS256 | 72h default | Time-limited sharing links |

**Functions:**
| Function | Description |
|----------|-------------|
| `create_qr_jwt(claims, config)` | Creates eternal QR JWT with diploma claims |
| `build_qr_url(token, config)` | Builds verification URL: `{base_url}?payload={jwt}` |
| `create_share_link_jwt(hash, ttl, config)` | Creates time-limited share-link JWT |
| `verify_qr_jwt(token, public_key)` | Verifies QR JWT |
| `verify_share_link_jwt(token, config)` | Verifies share-link JWT |

---

### [`src/db.rs`](src/db.rs)
**Database module root** - Declares submodules.

---

### [`src/db/models.rs`](src/db/models.rs)
**Database models** - Structs mapping to database tables.

**Models:**
| Struct | Table | Purpose |
|--------|-------|---------|
| `DiplomaHashRecord` | `diploma_hashes` | Hash + Ed25519 signature store |
| `BatchResultRecord` | `batch_results` | Per-diploma encrypted payload + QR JWT |
| `BatchRecord` | `batches` | Batch progress tracking |
| `UniversityRecord` | `universities` | University info and Ed25519 keys |
| `NewDiplomaHash` | - | Insert data for diploma_hashes |
| `NewBatchResult` | - | Insert data for batch_results |

---

### [`src/db/repository.rs`](src/db/repository.rs)
**Database operations** - All DB reads and writes.

**Functions:**
| Function | Operation | Description |
|----------|-----------|-------------|
| `insert_diploma_hash` | INSERT | Stores hash + signature (idempotent via ON CONFLICT) |
| `get_diploma_by_hash` | SELECT | Retrieves diploma record by hash |
| `insert_batch_result` | INSERT | Stores encrypted payload + QR JWT |
| `increment_batch_processed` | UPDATE | Increments batch counter |
| `set_batch_completed` | UPDATE | Marks batch as completed |
| `get_batch` | SELECT | Retrieves batch record |
| `get_university` | SELECT | Fetches university with Ed25519 private key |
| `is_batch_complete` | SELECT | Checks if all records processed |

---

### [`src/kafka.rs`](src/kafka.rs)
**Kafka module root** - Declares submodules.

---

### [`src/kafka/messages.rs`](src/kafka/messages.rs)
**Kafka message types** - Serde structs for Kafka topics.

**Inbound (from Gateway):**
| Struct | Field | Description |
|-------|-------|-------------|
| `DiplomaTask` | `batch_id` | Batch UUID |
| | `vuz_id` | University UUID |
| | `record_index` | Position in batch |
| | `total_in_batch` | Total records |
| | `student` | Student data |
| | `created_at` | Task timestamp |

**Outbound (to Gateway):**
| Struct | Field | Description |
|-------|-------|-------------|
| `ProcessingResult` | `diploma_hash` | SHA-256 hash |
| | `signature` | Ed25519 signature (base64) |
| | `encrypted_payload` | AES-GCM ciphertext (base64) |
| | `qr_payload` | QR verification URL |
| | `status` | Ok / Error |
| | `error` | Error message if failed |

---

### [`src/kafka/consumer.rs`](src/kafka/consumer.rs)
**Kafka consumer** - Wrapper around `rdkafka::StreamConsumer`.

**Responsibilities:**
- Subscribes to `diplomas.raw_tasks` topic
- Deserializes JSON to `DiplomaTask`
- Calls handler for each message
- Commits offset **only after** successful processing (at-least-once delivery)
- Commits invalid messages to avoid infinite retry

---

### [`src/kafka/producer.rs`](src/kafka/producer.rs)
**Kafka producer** - Wrapper around `rdkafka::FutureProducer`.

**Responsibilities:**
- Sends JSON-serialized messages to `diplomas.processing_results`
- Uses `diploma_hash` as message key for partition routing
- Handles async delivery with timeout

---

### [`src/handlers.rs`](src/handlers.rs)
**Handlers module root** - Declares the diploma handler submodule.

---

### [`src/handlers/diploma.rs`](src/handlers/diploma.rs)
**Diploma processing handler** - Orchestrates the full cryptographic pipeline.

**Processing steps:**
1. Fetch university record (private key) from DB
2. Generate random salt
3. Compute diploma hash (SHA-256)
4. Sign hash with Ed25519
5. Encrypt student payload with AES-GCM
6. Create QR JWT (EdDSA)
7. Build QR verification URL
8. Insert diploma_hash record
9. Insert batch_result record
10. Increment batch counter
11. Mark batch completed (if last record)
12. Send ProcessingResult to Kafka

**Error handling:** On any error, publishes `ProcessingResult { status: Error }` so Gateway can handle individual failures without stalling the batch.

---

## Database Tables

| Table | This service writes | This service reads | Owner |
|-------|---------------------|-------------------|-------|
| `diploma_hashes` | ✅ INSERT | — | Crypto Engine |
| `batch_results` | ✅ INSERT | — | Crypto Engine |
| `batches` | ✅ UPDATE | ✅ SELECT | Shared |
| `universities` | — | ✅ SELECT | Gateway |

---

## Environment Variables

```dotenv
# Kafka
APP__KAFKA__BROKERS=localhost:9092
APP__KAFKA__GROUP_ID=crypto-engine
APP__KAFKA__INPUT_TOPIC=diplomas.raw_tasks
APP__KAFKA__OUTPUT_TOPIC=diplomas.processing_results

# Postgres
APP__DATABASE__URL=postgres://user:pass@localhost:5432/diplomas_db

# JWT — university Ed25519 key (for QR tokens, eternal)
APP__JWT__QR_ED25519_PRIVATE_KEY_PEM="-----BEGIN PRIVATE KEY-----\n..."
# JWT — HMAC secret (for share-link tokens, HS256)
APP__JWT__AUTH_HMAC_SECRET=your-256-bit-secret-here

# Application
APP__APP__VERIFICATION_BASE_URL=https://platform.ru/verify?payload=
APP__APP__ENCRYPTION_KEY=<64-char hex string>

# Logging
RUST_LOG=crypto_engine=info,rdkafka=warn,sqlx=warn
```

---

## Running the Service

```bash
# Development
cargo run

# Production build
cargo build --release

# Run tests
cargo test

# Check compilation
cargo check

# Run with specific log level
RUST_LOG=debug cargo run
```

---

## Dependencies

| Crate | Purpose |
|-------|---------|
| `tokio` | Async runtime |
| `rdkafka` | Kafka client |
| `sqlx` | PostgreSQL with compile-time checks |
| `sha2` | SHA-256 hashing |
| `ed25519-dalek` | Ed25519 signatures |
| `aes-gcm` | AES-GCM encryption |
| `jsonwebtoken` | JWT handling |
| `serde` | Serialization |
| `config` | Configuration management |
| `tracing` | Structured logging |
