# Public Verification API

`pubver` — отдельный Go-сервис для публичной проверки дипломов по QR JWT и для ограниченного поиска по `diploma_number + vuz_code`.

Сервис работает по схеме:

1. Получает QR JWT в `GET /api/v1/verify?payload=<jwt>`.
2. Извлекает `vuz_id` без доверия к payload.
3. Загружает `universities.public_key`.
4. Проверяет `RS256` подпись JWT.
5. Пересчитывает `SHA-256(full_name|diploma_number|specialty|year|vuz_id|salt)`.
6. Сверяет результат с `sub` и `diploma_hash`.
7. Ищет хеш в `diploma_hashes`.
8. Возвращает статус диплома из БД.

Публичная верификация в этом сервисе не использует `diploma_hashes.signature` и не делает отдельную Ed25519-проверку.

## Что делает сервис

- проверяет диплом по QR JWT;
- ищет диплом по `vuz_code` и номеру;
- работает только на чтение;
- не пишет в Kafka;
- не загружает дипломы;
- не меняет статусы;
- не расшифровывает `encrypted_payload`.

## API

### `GET /healthz`

```json
{
  "status": "ok"
}
```

### `GET /api/v1/verify?payload=<jwt>`

Проверяет диплом по QR JWT.

Пример ответа:

```json
{
  "valid": true,
  "status": "active",
  "hash": "ad48ff40e10da83a32fcf59b1e4cc2db3ec06273238d4c4e3b693c86e901e875",
  "diploma_number": "DVS-2024-001234",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "bmstu",
  "year": null,
  "specialty": null
}
```

Варианты результата:

- `valid: true`, `status: active` — диплом найден и действителен;
- `valid: false`, `status: revoked` — диплом найден, но отозван;
- `valid: false`, `status: not_found` — хеш не найден в БД.

Ошибки:

- `400 Bad Request` — пустой `payload`, сломанный JWT, неизвестный `vuz_id`, неверная `RS256` подпись, mismatch хеша;
- `500 Internal Server Error` — внутренняя ошибка сервиса или БД.

### `GET /api/v1/verify/search?diploma_number=&vuz_code=`

Ищет диплом по `universities.vuz_code + diploma_hashes.diploma_number`.

Пример ответа:

```json
{
  "valid": true,
  "status": "active",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "bmstu",
  "year": null,
  "specialty": null
}
```

## QR JWT контракт

Ожидаемый header:

```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```

Ожидаемый payload:

```json
{
  "sub": "sha256-hash",
  "diploma_hash": "sha256-hash",
  "vuz_id": "550e8400-e29b-41d4-a716-446655440000",
  "diploma_number": "DVS-2024-001234",
  "student_name": "Ivanov Ivan Ivanovich",
  "specialty": "Software Engineering",
  "year": 2024,
  "salt": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  "iat": 1710000000,
  "exp": null
}
```

Обязательные claims:

- `vuz_id`
- `diploma_number`
- `student_name` или `full_name`
- `specialty`
- `year`
- `salt`

## Алгоритм хеширования

Используется одна и та же строка как в Crypto Engine, так и в `pubver`:

```text
full_name|diploma_number|specialty|year|vuz_id|salt
```

После этого считается:

```text
sha256(raw_string)
```

Результат хранится и сравнивается как lower-case hex string.

Реализация:

- [`hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go)

## JWT верификация

Сервис:

1. Извлекает `vuz_id`.
2. Загружает `universities.public_key`.
3. Проверяет `alg = RS256`.
4. Проверяет подпись JWT через RSA public key.

Поддерживаемые форматы RSA public key:

- PEM
- base64 DER
- hex DER

Реализация:

- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go)
- [`rs256.go`](/d:/diasoft/pubver/pkg/verifyhash/rs256.go)

## Временные заглушки `year` и `specialty`

Поля `year` и `specialty` сохранены в проекте:

- в доменных моделях;
- в JSON-ответах;
- в OpenAPI-контракте.

Но пока они не хранятся и не читаются из PostgreSQL. До согласования схемы БД сервис возвращает их как `null`.

Это сделано специально, чтобы:

- не ломать публичный контракт;
- не привязывать сервис к неподтвержденной схеме хранения;
- позже подключить реальные данные без смены API.

## Зависимости от БД

Сейчас сервис использует:

- `universities`
- `diploma_hashes`

Используемые поля `universities`:

- `id`
- `name`
- `public_key`
- `vuz_code`

Используемые поля `diploma_hashes`:

- `hash`
- `vuz_id`
- `diploma_number`
- `status`
- `revoked_at`

## Миграция `007`

[`007_public_verification_support.sql`](/d:/diasoft/pubver/migrations/007_public_verification_support.sql):

- добавляет `universities.vuz_code`;
- создает уникальный индекс по `vuz_code`;
- оставляет `year` и `specialty` как зарезервированные API-поля без добавления их в PostgreSQL.

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Назначение |
| --- | --- | --- | --- |
| `DATABASE_URL` | да | нет | строка подключения к PostgreSQL |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `REQUEST_TIMEOUT` | нет | `5s` | timeout на запрос |
| `LOG_LEVEL` | нет | `info` | уровень логирования |
| `DB_MAX_CONNS` | нет | `10` | лимит подключений к БД |

Пример находится в [.env.example](/d:/diasoft/pubver/.env.example).

## Локальный запуск

### Через Go

```powershell
cd d:\diasoft\pubver
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/diasoft?sslmode=disable"
go run ./cmd/pubver
```

### Через Docker Compose

```powershell
cd d:\diasoft\pubver
docker compose up --build
```

## Примеры запросов

```powershell
curl http://localhost:8080/healthz
curl "http://localhost:8080/api/v1/verify/search?diploma_number=DVS-2024-001234&vuz_code=bmstu"
curl "http://localhost:8080/api/v1/verify?payload=<jwt>"
```

## Структура проекта

- [`main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) — точка входа.
- [`config.go`](/d:/diasoft/pubver/internal/config/config.go) — env и defaults.
- [`model.go`](/d:/diasoft/pubver/internal/domain/model.go) — доменные модели.
- [`errors.go`](/d:/diasoft/pubver/internal/domain/errors.go) — доменные ошибки.
- [`router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) — HTTP routes.
- [`middleware.go`](/d:/diasoft/pubver/internal/httpapi/middleware.go) — request id, logging, recovery.
- [`context.go`](/d:/diasoft/pubver/internal/httpapi/context.go) — request id в context.
- [`verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) — бизнес-логика.
- [`repository.go`](/d:/diasoft/pubver/internal/repository/repository.go) — интерфейс репозитория.
- [`verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) — SQL к PostgreSQL.
- [`hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) — сбор строки и SHA-256.
- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go) — разбор QR JWT.
- [`rs256.go`](/d:/diasoft/pubver/pkg/verifyhash/rs256.go) — проверка `RS256`.
- [`public-verification.yaml`](/d:/diasoft/pubver/openapi/public-verification.yaml) — OpenAPI.
- [`architecture.md`](/d:/diasoft/pubver/docs/architecture.md) — архитектурное описание.

## Тестирование

Запуск:

```powershell
cd d:\diasoft\pubver
go test ./...
```

В проекте есть тесты для:

- JWT parsing;
- SHA-256 hashing;
- RS256 verification;
- service layer;
- HTTP handlers;
- middleware.

## Что можно сделать следующим шагом

- утвердить схему хранения `year` и `specialty`;
- подключить эти поля к PostgreSQL без смены API;
- добавить интеграционные тесты с реальной БД;
- добавить альтернативный `POST /api/v1/verify` с JWT в body, чтобы не передавать токен только через query parameter;
- добавить проверки `iss`, `aud`, `exp`, если они станут обязательными.
