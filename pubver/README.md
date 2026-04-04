# Public Verification API

`pubver` - отдельный Go-сервис для публичной проверки дипломов:

- по QR JWT через `GET /api/v1/verify?payload=<jwt>`
- по номеру диплома и коду ВУЗа через `GET /api/v1/verify/search?diploma_number=&vuz_code=`

Сервис работает только с реальной `PostgreSQL`.

## Как работает verify

`GET /api/v1/verify?payload=<jwt>`:

1. Без доверия читает из JWT только `vuz_id`.
2. Загружает `universities.public_key`.
3. Проверяет подпись JWT через `Ed25519` (`alg = EdDSA`).
4. Из валидного JWT читает верхнеуровневые claims:
   - `sub`
   - `diploma_hash`
   - `vuz_id`
   - `enc`
   - `iat`
5. Расшифровывает `enc` алгоритмом `A256GCM`.
6. Получает из расшифрованного JSON:
   - `full_name`
   - `diploma_number`
   - `specialty`
   - `degree`
   - `faculty`
   - `year`
   - `salt`
7. Пересчитывает:

```text
SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt)
```

8. Сверяет результат с `sub` и `diploma_hash`.
9. Ищет хеш в `diploma_hashes`.
10. Возвращает `active`, `revoked` или `not_found`.

## JWT контракт

### Header

```json
{
  "alg": "EdDSA",
  "typ": "JWT"
}
```

### Верхний payload JWT

```json
{
  "sub": "sha256-hash",
  "diploma_hash": "sha256-hash",
  "vuz_id": "550e8400-e29b-41d4-a716-446655440000",
  "enc": "<base64(nonce|ciphertext|tag)>",
  "iat": 1710000000
}
```

Обязательные верхние claims:

- `sub`
- `diploma_hash`
- `vuz_id`
- `enc`
- `iat`

### Что лежит внутри `enc`

`enc` - это `A256GCM` ciphertext, закодированный в base64.  
Формат байтов внутри строки:

```text
nonce(12 bytes) | ciphertext+tag
```

После расшифровки ожидается JSON:

```json
{
  "full_name": "Иванов Иван Иванович",
  "diploma_number": "ДВС-2024-001234",
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ФКН",
  "year": 2024,
  "salt": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
}
```

Обязательные поля внутри `enc`:

- `full_name`
- `diploma_number`
- `specialty`
- `degree`
- `faculty`
- `year`
- `salt`

## Алгоритмы

- подпись JWT: `Ed25519` (`EdDSA`)
- шифрование `enc`: `A256GCM`
- хеш диплома: `SHA-256`

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Назначение |
| --- | --- | --- | --- |
| `DATABASE_URL` | нет, если заданы `POSTGRES_*` | нет | строка подключения к PostgreSQL |
| `POSTGRES_HOST` | нет, если задан `DATABASE_URL` | нет | host PostgreSQL |
| `POSTGRES_PORT` | нет | `5432` | port PostgreSQL |
| `POSTGRES_DB` | нет, если задан `DATABASE_URL` | нет | имя базы |
| `POSTGRES_USER` | нет, если задан `DATABASE_URL` | нет | пользователь БД |
| `POSTGRES_PASSWORD` | нет | нет | пароль БД |
| `POSTGRES_SSLMODE` | нет | `disable` | режим SSL |
| `JWT_ENC_SECRET` | да | нет | секрет для `enc`, из которого ключ выводится так же, как в Crypto Service |
| `JWT_ENC_KEY_BASE64` | нет | нет | уже готовый 32-байтный AES-256 ключ в base64 |
| `JWT_ENC_KEY` | нет | нет | альтернативное имя той же base64-переменной |
| `JWT_ENC_KEY_HEX` | нет | нет | legacy fallback: уже готовый 32-байтный AES-256 ключ в hex |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `REQUEST_TIMEOUT` | нет | `5s` | timeout на запрос |
| `LOG_LEVEL` | нет | `info` | уровень логирования |
| `DB_MAX_CONNS` | нет | `10` | лимит подключений к БД |

Пример секрета `JWT_ENC_SECRET`:

```text
BTIXrJ+PmLGw8wXqFt37/RIq3PoF5HEXX7kQevleKMA=
```

## Локальный запуск

### Вариант 1. Через `DATABASE_URL`

```powershell
cd d:\diasoft\pubver
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/diasoft?sslmode=disable"
$env:JWT_ENC_SECRET="BTIXrJ+PmLGw8wXqFt37/RIq3PoF5HEXX7kQevleKMA="
go run ./cmd/pubver
```

### Вариант 2. Через `POSTGRES_*`

```powershell
cd d:\diasoft\pubver
$env:POSTGRES_HOST="localhost"
$env:POSTGRES_PORT="5432"
$env:POSTGRES_DB="diasoft"
$env:POSTGRES_USER="postgres"
$env:POSTGRES_PASSWORD="postgres"
$env:POSTGRES_SSLMODE="disable"
$env:JWT_ENC_SECRET="BTIXrJ+PmLGw8wXqFt37/RIq3PoF5HEXX7kQevleKMA="
go run ./cmd/pubver
```

## Примеры ответов

### `/api/v1/verify`

```json
{
  "valid": true,
  "status": "active",
  "hash": "4f4aa9c637fadb692fa4da544a0402a609253d7e28d4553a898efb9b430d0b26",
  "diploma_number": "ДВС-2024-001234",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "001X7276",
  "year": 2024,
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ФКН"
}
```

### `/api/v1/verify/search`

```json
{
  "valid": true,
  "status": "active",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "001X7276",
  "year": 2024,
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ФКН"
}
```

## Какие таблицы используются

- `universities`
- `diploma_hashes`
- `batch_results`
- `batch_record_attributes`

`verify`:

- статус и принадлежность диплома берет из `diploma_hashes`
- `university` и `vuz_code` берет из `universities`
- `year`, `specialty`, `degree`, `faculty` возвращает из расшифрованного `enc`

`search`:

- ищет по `universities.vuz_code + diploma_hashes.diploma_number`
- читает `year`, `specialty`, `degree`, `faculty` из `batch_record_attributes`
  через `batch_results.diploma_hash -> (batch_id, record_index)`

## Структура проекта

- [`main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) - точка входа
- [`config.go`](/d:/diasoft/pubver/internal/config/config.go) - env и конфиг
- [`router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) - HTTP-ручки
- [`verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) - бизнес-логика
- [`verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) - SQL к PostgreSQL
- [`ed25519.go`](/d:/diasoft/pubver/pkg/verifyhash/ed25519.go) - проверка подписи JWT
- [`a256gcm.go`](/d:/diasoft/pubver/pkg/verifyhash/a256gcm.go) - расшифровка `enc`
- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go) - разбор верхних claims JWT
- [`hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) - пересчет SHA-256
- [`public-verification.yaml`](/d:/diasoft/pubver/openapi/public-verification.yaml) - OpenAPI

## Проверка

```powershell
cd d:\diasoft\pubver
go test ./...
```
