# Public Verification API

`pubver` — отдельный Go-сервис для публичной проверки дипломов:

- по QR JWT через `GET /api/v1/verify?payload=<jwt>`
- по номеру диплома и коду ВУЗа через `GET /api/v1/verify/search?diploma_number=&vuz_code=`

Сервис read-only:

- не пишет в Kafka
- не загружает дипломы
- не меняет статусы дипломов
- не расшифровывает `encrypted_payload`

## Как работает проверка

`GET /api/v1/verify?payload=<jwt>` делает следующее:

1. Извлекает `vuz_id` из JWT payload без доверия к данным.
2. Загружает `universities.public_key`.
3. Проверяет `RS256` подпись JWT.
4. Пересчитывает `SHA-256(full_name|diploma_number|specialty|year|vuz_id|salt)`.
5. Сверяет пересчитанный хеш с `sub` и `diploma_hash`.
6. Ищет хеш в `diploma_hashes`.
7. Возвращает статус диплома.

Публичная верификация не использует `diploma_hashes.signature` и не делает отдельную Ed25519-проверку.

## API

### `GET /healthz`

Ответ:

```json
{
  "status": "ok"
}
```

### `GET /api/v1/verify?payload=<jwt>`

Пример успешного ответа:

```json
{
  "valid": true,
  "status": "active",
  "hash": "4f4aa9c637fadb692fa4da544a0402a609253d7e28d4553a898efb9b430d0b26",
  "diploma_number": "ДВС-2024-001234",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "001X7276",
  "year": null,
  "specialty": null
}
```

`vuz_code` — публичный код ВУЗа в формате наподобие `001X7276`, то есть код по сводному реестру. Именно такой код и должен храниться в `universities.vuz_code`.

Варианты результата:

- `valid: true`, `status: active`
- `valid: false`, `status: revoked`
- `valid: false`, `status: not_found`

Ошибки:

- `400 Bad Request` — пустой `payload`, сломанный JWT, неизвестный `vuz_id`, неверная `RS256` подпись, mismatch хеша
- `500 Internal Server Error` — внутренняя ошибка сервиса или БД

### `GET /api/v1/verify/search?diploma_number=&vuz_code=`

Ищет диплом по `universities.vuz_code + diploma_hashes.diploma_number`.

Пример ответа:

```json
{
  "valid": true,
  "status": "active",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "001X7276",
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
  "diploma_number": "ДВС-2024-001234",
  "student_name": "Иванов Иван Иванович",
  "specialty": "Программная инженерия",
  "year": 2024,
  "salt": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  "iat": 1710000000
}
```

Обязательные claims:

- `vuz_id`
- `diploma_number`
- `student_name` или `full_name`
- `specialty`
- `year`
- `salt`

Дополнительные claims:

- `sub`
- `diploma_hash`
- `iat`
- `exp`

## Алгоритм хеширования

`pubver` и Crypto Engine должны использовать одну и ту же строку:

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

## Проверка JWT подписи

Сервис:

1. Извлекает `vuz_id`
2. Загружает `universities.public_key`
3. Проверяет `alg = RS256`
4. Проверяет подпись JWT через RSA public key

Поддерживаемые форматы RSA public key:

- PEM
- base64 DER
- hex DER

Реализация:

- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go)
- [`rs256.go`](/d:/diasoft/pubver/pkg/verifyhash/rs256.go)

## Временные заглушки `year` и `specialty`

Поля `year` и `specialty` сохранены:

- в доменных моделях
- в JSON-ответах
- в OpenAPI-контракте

Но пока они не хранятся и не читаются из PostgreSQL. До согласования схемы БД сервис возвращает их как `null`.

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

## Stub-режим без PostgreSQL

Для локальной ручной проверки можно запустить сервис без БД:

```powershell
cd d:\diasoft\pubver
$env:USE_STUB_DATA="true"
go run ./cmd/pubver
```

В этом режиме:

- PostgreSQL не используется
- `verify` и `search` работают на встроенных stub-данных
- сервис печатает в лог готовые примеры для Postman:
  - `search_active_url`
  - `search_revoked_url`
  - `verify_active_url`
  - `verify_revoked_url`

То есть после старта можно просто взять URL из лога и отправить в Postman.

Важно:

- `healthz` в stub-режиме тоже работает
- `year` и `specialty` остаются `null`
- боевой режим с реальной БД не меняется

## Миграция `007`

[`007_public_verification_support.sql`](/d:/diasoft/pubver/migrations/007_public_verification_support.sql):

- добавляет `universities.vuz_code`
- создает уникальный индекс по `vuz_code`
- оставляет `year` и `specialty` как зарезервированные API-поля без добавления их в PostgreSQL

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Назначение |
| --- | --- | --- | --- |
| `DATABASE_URL` | да, кроме stub-режима | нет | строка подключения к PostgreSQL |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `REQUEST_TIMEOUT` | нет | `5s` | timeout на запрос |
| `LOG_LEVEL` | нет | `info` | уровень логирования |
| `DB_MAX_CONNS` | нет | `10` | лимит подключений к БД |
| `USE_STUB_DATA` | нет | `false` | запуск без PostgreSQL на встроенных stub-данных |

Пример находится в [.env.example](/d:/diasoft/pubver/.env.example).

## Локальный запуск

### Боевой режим с PostgreSQL

```powershell
cd d:\diasoft\pubver
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/diasoft?sslmode=disable"
go run ./cmd/pubver
```

### Stub-режим без PostgreSQL

```powershell
cd d:\diasoft\pubver
$env:USE_STUB_DATA="true"
go run ./cmd/pubver
```

## Примеры запросов

### Healthcheck

```powershell
curl http://localhost:8080/healthz
```

### Search

```powershell
curl "http://localhost:8080/api/v1/verify/search?diploma_number=ДВС-2024-001234&vuz_code=001X7276"
```

### Verify

```powershell
curl "http://localhost:8080/api/v1/verify?payload=<jwt>"
```

В stub-режиме удобнее брать готовые `verify_*_url` прямо из логов сервиса.

## Структура проекта

- [`main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) — точка входа
- [`config.go`](/d:/diasoft/pubver/internal/config/config.go) — env и defaults
- [`model.go`](/d:/diasoft/pubver/internal/domain/model.go) — доменные модели
- [`errors.go`](/d:/diasoft/pubver/internal/domain/errors.go) — доменные ошибки
- [`router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) — HTTP routes
- [`middleware.go`](/d:/diasoft/pubver/internal/httpapi/middleware.go) — request id, logging, recovery
- [`context.go`](/d:/diasoft/pubver/internal/httpapi/context.go) — request id в context
- [`verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) — бизнес-логика
- [`repository.go`](/d:/diasoft/pubver/internal/repository/repository.go) — интерфейс репозитория
- [`verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) — PostgreSQL-реализация
- [`verification_repository.go`](/d:/diasoft/pubver/internal/repository/stub/verification_repository.go) — stub-реализация без БД
- [`hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) — сбор строки и SHA-256
- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go) — разбор QR JWT
- [`rs256.go`](/d:/diasoft/pubver/pkg/verifyhash/rs256.go) — проверка `RS256`
- [`public-verification.yaml`](/d:/diasoft/pubver/openapi/public-verification.yaml) — OpenAPI
- [`architecture.md`](/d:/diasoft/pubver/docs/architecture.md) — архитектурное описание

## Тестирование

Запуск:

```powershell
cd d:\diasoft\pubver
go test ./...
```

Покрыты:

- JWT parsing
- SHA-256 hashing
- RS256 verification
- service layer
- HTTP handlers
- middleware

## Что можно сделать следующим шагом

- утвердить схему хранения `year` и `specialty`
- подключить эти поля к PostgreSQL без смены API
- добавить интеграционные тесты с реальной БД
- добавить альтернативный `POST /api/v1/verify` с JWT в body, чтобы не передавать токен только через query parameter
- добавить проверки `iss`, `aud`, `exp`, если они станут обязательными
