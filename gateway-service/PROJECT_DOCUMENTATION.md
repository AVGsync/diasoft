# Документация Gateway Service

## 1. Назначение сервиса

`gateway-service` — это точка входа для ВУЗов и администраторов платформы.

Сервис отвечает за:

- регистрацию и аутентификацию ВУЗов;
- ручное одобрение ВУЗов администратором платформы;
- управление API-ключами для ERP-интеграций;
- приём реестров дипломов в JSON и CSV;
- публикацию задач обработки в Kafka;
- приём результатов обработки из Kafka;
- хранение хешей дипломов, QR payload и статусов батчей;
- генерацию share-link для студента и QR-кодов для временного доступа;
- публичную проверку диплома по QR;
- приём и хранение зашифрованного `private_key` ВУЗа для `Processing Engine`.

`Processing Engine` не имеет внешнего HTTP API. Он общается с `gateway-service` только через Kafka и самостоятельно читает ключи подписи из PostgreSQL.

## 2. Архитектура

### 2.1 Сервисы

`Gateway Service`

- публичный HTTP API для ВУЗов, студентов и администраторов;
- пишет задачи в `diplomas.raw_tasks`;
- читает результаты из `diplomas.processing_results`;
- хранит зашифрованные signing key ВУЗов в PostgreSQL.

`Processing Engine`

- Kafka worker без внешнего API;
- читает `diplomas.raw_tasks`;
- загружает зашифрованный `private_key` из PostgreSQL по `vuz_id`;
- расшифровывает ключ общим master key;
- считает хеш диплома;
- формирует и подписывает `qr_payload` через `Ed25519`;
- пишет результаты в `diplomas.processing_results`.

`PostgreSQL`

- основная БД платформы;
- хранит ВУЗы, API-ключи, батчи, результаты обработки, хеши дипломов, share links и зашифрованные signing key.

`Kafka`

- транспорт между `Gateway Service` и `Processing Engine`;
- используется два топика:
  - `diplomas.raw_tasks`
  - `diplomas.processing_results`

## 3. Криптографическая модель

### 3.1 Что именно хешируется

Хеш диплома строится из набора полей:

- `full_name`
- `diploma_number`
- `specialty`
- `year`
- `vuz_id`
- `salt`

Каноническая строка для расчёта:

```text
full_name|diploma_number|specialty|year|vuz_id|salt
```

Алгоритм:

```text
SHA-256
```

Результат сохраняется в:

- `diploma_hashes.hash`

### 3.2 Что именно подписывается

Асимметрично подписывается только QR JWT.

Алгоритм подписи:

- `Ed25519`
- в JWT заголовке используется `alg = EdDSA`

Подписывает:

- `Processing Engine`

Отдельной подписи для `diploma_hash` больше нет.

Колонка `signature` в `diploma_hashes` не используется и не нужна.

### 3.3 Что именно шифруется

Шифруется приватный ключ ВУЗа перед записью в PostgreSQL.

Алгоритм шифрования:

- `AES-256-GCM`

Поток:

- `Gateway Service` принимает `private_key_pem`;
- шифрует его перед сохранением;
- `Processing Engine` расшифровывает ключ только в памяти перед подписью QR JWT.

Общий конфиг:

- `SIGNING_KEYS_MASTER_KEY`
- base64-строка, которая после декодирования даёт ровно 32 байта

### 3.4 Почему нельзя передавать `private_key` через Kafka

`private_key` не передаётся через Kafka, потому что:

- сообщения Kafka хранятся по retention;
- содержимое топиков можно увидеть через Kafka UI;
- ключ будет дублироваться в ретраях, логах и дампах;
- любой consumer топика сможет его прочитать.

Правильный путь такой:

1. ВУЗ загружает `private_key` в `Gateway Service` через HTTP.
2. `Gateway Service` сохраняет его в PostgreSQL в зашифрованном виде.
3. `Processing Engine` читает ключ из БД по `vuz_id`.

## 4. JWT в системе

### 4.1 Access JWT

Назначение:

- логин ВУЗа;
- логин администратора платформы.

Алгоритм:

- `HS256`

Пример claims:

```json
{
  "sub": "uuid-subject",
  "vuz_id": "uuid-vuz",
  "email": "rector@example.ru",
  "role": "university",
  "status": "active",
  "iat": 1710000000,
  "exp": 1710086400
}
```

### 4.2 Share JWT

Назначение:

- временная ссылка для студента.

Алгоритм:

- `HS256`

Пример claims:

```json
{
  "sub": "sha256-hash",
  "diploma_hash": "a3f9c2b1...",
  "type": "share_link",
  "iat": 1710000000,
  "exp": 1710259200
}
```

### 4.3 QR JWT

Назначение:

- payload, вшиваемый в QR-код диплома.

Алгоритм:

- `EdDSA`

Подписывает:

- `Processing Engine`

Пример claims:

```json
{
  "sub": "sha256-hash",
  "diploma_hash": "a3f9c2b1...",
  "vuz_id": "uuid-vuz",
  "diploma_number": "DVS-2024-001234",
  "student_name": "Иванов Иван Иванович",
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ФКН",
  "year": 2024,
  "salt": "random-32-byte-hex",
  "iat": 1710000000
}
```

Правила верификации:

- JWT должен быть подписан `Ed25519`;
- публичный ключ берётся из `universities.public_key`;
- сервис пересчитывает `SHA-256` по claims и сверяет с `diploma_hashes.hash`;
- диплом должен существовать в БД и не быть отозван.

## 5. HTTP API

### 5.1 Аутентификация

#### `POST /api/v1/auth/register`

Назначение:

- создать заявку ВУЗа на подключение.

Request:

```json
{
  "name": "МГУ",
  "vuz_code": "mgu2024a",
  "inn": "7707083893",
  "ogrn": "1027700132195",
  "email": "rector@mgu.ru",
  "password": "strong-password"
}
```

Примечания:

- `public_key` не передаётся при регистрации;
- public key вычисляется gateway из загруженного `private_key_pem` при `PUT /api/v1/vuz/signing-key`.

Response `201 Created`:

```json
{
  "id": "uuid",
  "status": "pending",
  "created_at": "2026-04-04T12:00:00Z"
}
```

#### `POST /api/v1/auth/login`

Response `200 OK`:

```json
{
  "access_token": "jwt",
  "token_type": "Bearer",
  "expires_at": "2026-04-05T12:00:00Z",
  "role": "university",
  "status": "active",
  "vuz_id": "uuid",
  "email": "rector@mgu.ru"
}
```

### 5.2 Администрирование

#### `POST /api/v1/admin/universities/{id}/approve`

Назначение:

- активировать заявку ВУЗа.

Авторизация:

- `Bearer <admin jwt>`

#### `GET /api/v1/admin/stats`

Назначение:

- вернуть статистику для дашборда платформы.

Авторизация:

- `Bearer <admin jwt>`

### 5.3 API-ключи ВУЗа

#### `POST /api/v1/vuz/api-keys`

Назначение:

- создать статический API-ключ для ERP-интеграции.

Авторизация:

- `Bearer <university jwt>`

Request:

```json
{
  "name": "ERP integration"
}
```

Response `201 Created`:

```json
{
  "id": "uuid",
  "name": "ERP integration",
  "api_key": "vuz_...",
  "created_at": "2026-04-04T12:00:00Z"
}
```

#### `GET /api/v1/vuz/api-keys`

Назначение:

- получить список активных API-ключей.

Авторизация:

- `Bearer <university jwt>`

### 5.4 Управление signing key ВУЗа

#### `PUT /api/v1/vuz/signing-key`

Назначение:

- загрузить или заменить `Ed25519 private key` ВУЗа.

Авторизация:

- `Bearer <university jwt>`

Request:

```json
{
  "private_key_pem": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"
}
```

Поведение:

1. gateway парсит `private_key_pem` как `Ed25519`;
2. выводит из него public key;
3. если `universities.public_key` уже заполнен, ключи должны совпасть;
4. если `universities.public_key` пуст, gateway записывает туда вычисленный public key;
5. gateway шифрует `private_key_pem` через `AES-256-GCM`;
6. зашифрованный ключ сохраняется в `university_signing_keys`.

Response `201 Created`:

```json
{
  "configured": true,
  "key_algorithm": "ed25519",
  "encryption_algorithm": "aes-256-gcm",
  "public_key_fingerprint": "hex-sha256",
  "updated_at": "2026-04-04T12:00:00Z"
}
```

Ошибки:

- `400` — некорректный `Ed25519 private key`;
- `400` — private key не соответствует public key ВУЗа;
- `404` — ВУЗ не найден.

#### `GET /api/v1/vuz/signing-key`

Назначение:

- получить metadata о текущем signing key.

Авторизация:

- `Bearer <university jwt>`

Response `200 OK`:

```json
{
  "configured": true,
  "key_algorithm": "ed25519",
  "encryption_algorithm": "aes-256-gcm",
  "public_key_fingerprint": "hex-sha256",
  "updated_at": "2026-04-04T12:00:00Z"
}
```

### 5.5 Работа с дипломами

#### `POST /api/v1/diplomas/upload`

Назначение:

- загрузить батч дипломов.

Авторизация:

- `Bearer <university jwt>`
- или `ApiKey <key>`

JSON request:

```json
{
  "diplomas": [
    {
      "full_name": "Иванов Иван Иванович",
      "diploma_number": "ДВС-2024-001234",
      "specialty": "Программная инженерия",
      "degree": "Бакалавр",
      "faculty": "ФКН",
      "year": 2024
    }
  ]
}
```

CSV headers:

```text
full_name,diploma_number,specialty,degree,faculty,year
```

Response `202 Accepted`:

```json
{
  "batch_id": "uuid",
  "status": "processing"
}
```

#### `GET /api/v1/diplomas/batches/{batch_id}`

Назначение:

- получить прогресс обработки батча.

Response `200 OK`:

```json
{
  "id": "uuid",
  "vuz_id": "uuid",
  "status": "processing",
  "total_records": 500,
  "processed_records": 450,
  "failed_records": 3,
  "created_at": "2026-04-04T12:00:00Z",
  "completed_at": null
}
```

#### `GET /api/v1/diplomas/batches/{batch_id}/download`

Назначение:

- собрать Excel-файл со студентами и QR-кодами.

Источник строк:

- `batch_results.qr_payload`

Открытые строки студентов в PostgreSQL не хранятся.

#### `PATCH /api/v1/diplomas/{diploma_hash}/revoke`

Назначение:

- отозвать диплом.

Эффект:

- `UPDATE diploma_hashes SET status = 'revoked', revoked_at = NOW()`

### 5.6 Личный кабинет студента

#### `GET /api/v1/student/search?diploma_number=&full_name=`

Назначение:

- найти студента по номеру диплома и/или ФИО.

Источник данных:

- `diploma_hashes`
- `batch_results.qr_payload`

#### `POST /api/v1/student/share`

Request:

```json
{
  "diploma_hash": "64-char-sha256",
  "ttl_hours": 72
}
```

Response:

```json
{
  "share_url": "http://localhost:8080/api/v1/student/share/<token>",
  "token": "<jwt>",
  "expires_at": "2026-04-07T12:00:00Z"
}
```

#### `GET /api/v1/student/qr?diploma_hash=&format=png|svg&ttl_hours=72`

Назначение:

- сгенерировать QR для share-link.

#### `GET /api/v1/student/share/{token}`

Назначение:

- развернуть share token и вернуть снапшот диплома.

### 5.7 Публичная верификация

#### `GET /api/v1/verify?payload=<qr_jwt>`

Назначение:

- проверить диплом по QR JWT.

Шаги проверки:

1. распарсить QR JWT без доверия;
2. извлечь claims;
3. загрузить `public_key` ВУЗа;
4. проверить подпись JWT через `Ed25519`;
5. пересчитать `SHA-256(full_name|diploma_number|specialty|year|vuz_id|salt)`;
6. сравнить результат с `diploma_hashes.hash`;
7. проверить статус диплома в `diploma_hashes`.

Response:

```json
{
  "valid": true,
  "status": "active",
  "diploma_hash": "a3f9c2b1...",
  "diploma_number": "ДВС-2024-001234",
  "student_name": "Иванов Иван Иванович",
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ФКН",
  "year": 2024,
  "university_id": "uuid",
  "university": "МГУ",
  "hash_matches": true,
  "jwt_signature_valid": true,
  "created_at": "2026-04-04T12:00:00Z",
  "message": "diploma is valid"
}
```

## 6. Kafka-контракты

### 6.1 Топик `diplomas.raw_tasks`

Producer:

- `Gateway Service`

Consumer:

- `Processing Engine`

Сообщение:

```json
{
  "batch_id": "uuid-batch",
  "vuz_id": "uuid-vuz",
  "record_index": 42,
  "total_in_batch": 500,
  "student": {
    "full_name": "Иванов Иван Иванович",
    "diploma_number": "ДВС-2024-001234",
    "specialty": "Программная инженерия",
    "degree": "Бакалавр",
    "faculty": "ФКН",
    "year": 2024,
    "raw_csv_row": "..."
  },
  "created_at": "2026-04-04T12:00:00Z"
}
```

Важно:

- `private_key` не передаётся через Kafka;
- `public_key` тоже не передаётся через Kafka;
- `Processing Engine` сам читает signing key из PostgreSQL по `vuz_id`.

### 6.2 Топик `diplomas.processing_results`

Producer:

- `Processing Engine`

Consumer:

- `Gateway Service`

Успешное сообщение:

```json
{
  "batch_id": "uuid-batch",
  "vuz_id": "uuid-vuz",
  "record_index": 42,
  "diploma_hash": "a3f9c2b1d4e5...",
  "qr_payload": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "status": "ok",
  "error": null,
  "processed_at": "2026-04-04T12:00:01Z"
}
```

Сообщение об ошибке:

```json
{
  "batch_id": "uuid-batch",
  "vuz_id": "uuid-vuz",
  "record_index": 42,
  "diploma_hash": "",
  "qr_payload": null,
  "status": "error",
  "error": "failed to sign qr payload",
  "processed_at": "2026-04-04T12:00:01Z"
}
```

Отдельного поля `signature` в result message больше нет.

## 7. Итоговая схема PostgreSQL

### 7.1 `universities`

- `id UUID PK`
- `vuz_code VARCHAR(20) UNIQUE`
- `name VARCHAR(255)`
- `inn VARCHAR(12) UNIQUE`
- `ogrn VARCHAR(15) UNIQUE`
- `email VARCHAR(255) UNIQUE`
- `password_hash VARCHAR(255)`
- `public_key TEXT`
- `status VARCHAR(20)` -> `pending | active | blocked`
- `created_at TIMESTAMPTZ`

### 7.2 `platform_admins`

- `id UUID PK`
- `email VARCHAR(255) UNIQUE`
- `password_hash VARCHAR(255)`
- `created_at TIMESTAMPTZ`

### 7.3 `api_keys`

- `id UUID PK`
- `vuz_id UUID FK -> universities(id)`
- `key_hash VARCHAR(255) UNIQUE`
- `name VARCHAR(100)`
- `is_active BOOLEAN`
- `created_at TIMESTAMPTZ`

### 7.4 `batches`

- `id UUID PK`
- `vuz_id UUID FK -> universities(id)`
- `status VARCHAR(20)` -> `processing | completed | failed`
- `total_records INTEGER`
- `processed_records INTEGER`
- `created_at TIMESTAMPTZ`
- `completed_at TIMESTAMPTZ`

### 7.5 `batch_results`

- `id UUID PK`
- `batch_id UUID FK -> batches(id)`
- `record_index INTEGER`
- `diploma_hash VARCHAR(64) NULL`
- `qr_payload TEXT NULL`
- `status VARCHAR(20)` -> `ok | error`
- `error TEXT NULL`
- `created_at TIMESTAMPTZ`

Ограничения:

- `UNIQUE(batch_id, record_index)`
- `UNIQUE(diploma_hash)`

### 7.6 `diploma_hashes`

- `hash VARCHAR(64) PK`
- `vuz_id UUID FK -> universities(id)`
- `diploma_number VARCHAR(50)`
- `status VARCHAR(20)` -> `active | revoked`
- `created_at TIMESTAMPTZ`
- `revoked_at TIMESTAMPTZ`

Ограничения:

- `UNIQUE(vuz_id, diploma_number)`

Колонки `signature` нет.

### 7.7 `share_links`

- `id UUID PK`
- `diploma_hash VARCHAR(64) FK -> diploma_hashes(hash)`
- `token TEXT UNIQUE`
- `expires_at TIMESTAMPTZ`
- `used_count INTEGER`
- `created_at TIMESTAMPTZ`

### 7.8 `university_signing_keys`

- `vuz_id UUID PK FK -> universities(id)`
- `encrypted_private_key TEXT`
- `key_algorithm VARCHAR(20)` -> `ed25519`
- `encryption_algorithm VARCHAR(50)` -> `aes-256-gcm`
- `public_key_fingerprint VARCHAR(64)`
- `created_at TIMESTAMPTZ`
- `updated_at TIMESTAMPTZ`

## 8. Конфигурация

Ключевые runtime-настройки:

- `DATABASE_URL`
- `JWT_SECRET`
- `SHARE_JWT_SECRET`
- `PUBLIC_BASE_URL`
- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`
- `KAFKA_BROKERS`
- `KAFKA_RAW_TOPIC`
- `KAFKA_RESULTS_TOPIC`
- `KAFKA_CONSUMER_GROUP`
- `SIGNING_KEYS_MASTER_KEY`

Требования к `SIGNING_KEYS_MASTER_KEY`:

- base64-строка;
- после декодирования должна давать ровно 32 байта;
- значение должно совпадать у `Gateway Service` и `Processing Engine`.

## 9. Ответственность Processing Engine

Для каждой Kafka-задачи worker должен:

1. прочитать задачу из `diplomas.raw_tasks`;
2. загрузить зашифрованный private key из `university_signing_keys` по `vuz_id`;
3. расшифровать ключ с помощью `SIGNING_KEYS_MASTER_KEY`;
4. сгенерировать `salt`;
5. посчитать `diploma_hash` через `SHA-256`;
6. собрать claims для QR JWT;
7. подписать QR JWT через `Ed25519`;
8. отправить результат в `diplomas.processing_results`.

## 10. Полный workflow

### 10.1 Онбординг ВУЗа

1. ВУЗ вызывает `POST /api/v1/auth/register`.
2. В БД создаётся запись ВУЗа со статусом `pending`.
3. Администратор вызывает `POST /api/v1/admin/universities/{id}/approve`.
4. ВУЗ логинится через `POST /api/v1/auth/login`.

### 10.2 Настройка signing key

1. ВУЗ генерирует пару ключей `Ed25519` на своей стороне.
2. ВУЗ не передаёт `public_key` при регистрации.
3. ВУЗ загружает `private_key_pem` через `PUT /api/v1/vuz/signing-key`.
4. Gateway вычисляет из private key соответствующий public key и сохраняет его в `universities.public_key`.
5. Gateway шифрует private key через `AES-256-GCM`.
6. Gateway сохраняет его в `university_signing_keys`.

### 10.3 Обработка батча дипломов

1. ВУЗ загружает JSON или CSV через `POST /api/v1/diplomas/upload`.
2. Gateway создаёт запись в `batches`.
3. Gateway публикует по одной задаче на диплом в `diplomas.raw_tasks`.
4. `Processing Engine` читает задачу.
5. `Processing Engine` загружает и расшифровывает private key ВУЗа из PostgreSQL.
6. `Processing Engine` генерирует `salt`.
7. `Processing Engine` считает `diploma_hash = SHA-256(full_name|diploma_number|specialty|year|vuz_id|salt)`.
8. `Processing Engine` собирает claims для QR JWT.
9. `Processing Engine` подписывает QR JWT через `Ed25519`.
10. `Processing Engine` отправляет результат в `diplomas.processing_results`.
11. Gateway читает результат.
12. Gateway записывает:
    - `diploma_hashes`
    - `batch_results`
13. Gateway обновляет прогресс батча в `batches`.

### 10.4 Выгрузка и student flow

1. ВУЗ вызывает `GET /api/v1/diplomas/batches/{batch_id}/download`.
2. Gateway восстанавливает поля студента из `batch_results.qr_payload`.
3. Gateway собирает Excel и QR-картинки на лету.
4. Поиск студента и share-link flow также используют `diploma_hashes` и `batch_results.qr_payload`.

### 10.5 Публичная проверка QR

1. Сканер вызывает `GET /api/v1/verify?payload=<qr_jwt>`.
2. Gateway извлекает claims из QR JWT.
3. Gateway загружает `public_key` ВУЗа.
4. Gateway проверяет подпись JWT через `Ed25519`.
5. Gateway пересчитывает `SHA-256` по данным из claims.
6. Gateway ищет хеш в `diploma_hashes`.
7. Gateway проверяет статус диплома:
   - `active`
   - `revoked`
8. Gateway возвращает результат верификации.
