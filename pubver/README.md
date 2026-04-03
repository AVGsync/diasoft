# Public Verification API

`pubver` — отдельный Go-сервис для публичной проверки дипломов по QR JWT и для ограниченного поиска по номеру диплома и коду ВУЗа.

Финальная схема проверки в этом сервисе такая:

1. QR содержит JWT, подписанный алгоритмом `RS256`.
2. Сервис получает `vuz_id` из JWT payload.
3. По `vuz_id` сервис находит `universities.public_key`.
4. Этим ключом проверяет подпись JWT.
5. Из проверенного payload пересчитывает `SHA-256`.
6. Сравнивает пересчитанный хеш с `sub` или `diploma_hash`.
7. Ищет хеш в `diploma_hashes`.
8. Проверяет статус диплома в БД.

Сервис не использует отдельную проверку подписи записи диплома. Для публичной верификации используются только:

- `RS256` подпись самого JWT;
- пересчет хеша;
- lookup в БД.

## Назначение сервиса

`pubver` решает две задачи:

1. Публичная проверка диплома по QR JWT.
2. Поиск диплома по `diploma_number + vuz_code`.

Сервис работает в режиме чтения:

- не загружает дипломы;
- не пишет в Kafka;
- не создает батчи;
- не шифрует и не подписывает данные;
- не меняет статус диплома;
- не расшифровывает `encrypted_payload`.

## Место в общей архитектуре

Поток системы выглядит так:

1. ВУЗ загружает дипломы в основной API.
2. Основной API отправляет задачи в Kafka `diplomas.raw_tasks`.
3. Crypto Engine обрабатывает запись, генерирует `salt`, считает `SHA-256`, формирует `qr_payload` и подписывает JWT приватным RSA-ключом ВУЗа.
4. Основной API сохраняет результаты в PostgreSQL.
5. Публичный сервис `pubver` читает данные из PostgreSQL и отвечает на внешние запросы.

Логически `pubver` — отдельный read-only сервис, который можно:

- вынести за API Gateway;
- подключить к read replica PostgreSQL;
- ограничить rate limiting и WAF;
- отделить от приватного API ВУЗов.

## API

### `GET /healthz`

Healthcheck endpoint.

Ответ:

```json
{
  "status": "ok"
}
```

### `GET /api/v1/verify?payload=<jwt>`

Публичная проверка диплома по QR JWT.

Что делает endpoint:

1. Принимает JWT из query-параметра `payload`.
2. Декодирует payload без доверия к данным.
3. Извлекает `vuz_id`.
4. Находит RSA public key ВУЗа в `universities.public_key`.
5. Проверяет подпись JWT алгоритмом `RS256`.
6. Из проверенного payload извлекает данные диплома.
7. Пересчитывает `SHA-256`.
8. Сравнивает хеш с `sub` и `diploma_hash`, если они есть в токене.
9. Ищет запись в `diploma_hashes.hash`.
10. Возвращает результат проверки.

Пример успешного ответа:

```json
{
  "valid": true,
  "status": "active",
  "hash": "ad48ff40e10da83a32fcf59b1e4cc2db3ec06273238d4c4e3b693c86e901e875",
  "diploma_number": "DVS-2024-001234",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "bmstu",
  "year": 2024,
  "specialty": "Software Engineering"
}
```

Варианты результата:

- `valid: true`, `status: active` — диплом найден и действителен;
- `valid: false`, `status: revoked` — диплом найден, но отозван;
- `valid: false`, `status: not_found` — хеш не найден в БД.

Ошибки:

- `400 Bad Request` — пустой `payload`, невалидная структура JWT, неизвестный `vuz_id`, неверная `RS256` подпись, mismatch хеша;
- `500 Internal Server Error` — ошибка БД или внутренняя ошибка сервиса.

### `GET /api/v1/verify/search?diploma_number=&vuz_code=`

Публичный поиск диплома по номеру диплома и коду ВУЗа.

Что делает endpoint:

1. Принимает `diploma_number`.
2. Принимает `vuz_code`.
3. Ищет запись по `universities.vuz_code + diploma_hashes.diploma_number`.
4. Возвращает публичный статус и безопасные метаданные.

Пример ответа:

```json
{
  "valid": true,
  "status": "active",
  "university": "Bauman Moscow State Technical University",
  "vuz_code": "bmstu",
  "year": 2024,
  "specialty": "Software Engineering"
}
```

## Контракт QR JWT

Сервис ожидает JWT с header:

```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```

Payload ожидается в плоском виде:

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
- `student_name`
  Альтернатива: `full_name`
- `specialty`
- `year`
- `salt`

Дополнительные claims:

- `sub`
- `diploma_hash`
- `iat`
- `exp`

Сервис не использует `exp` как решающий признак валидности диплома. В вашей модели отзыв идет через `diploma_hashes.status`.

## Алгоритм хеширования

Crypto Engine и `pubver` должны использовать один и тот же алгоритм.

Входные данные:

- `full_name`
- `diploma_number`
- `specialty`
- `year`
- `vuz_id`
- `salt`

Сервис собирает строку:

```text
full_name|diploma_number|specialty|year|vuz_id|salt
```

После этого считает:

```text
sha256(raw_string)
```

Итог хранится и сравнивается как hex-строка в нижнем регистре.

Реализация:

- [`pkg/verifyhash/hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go)

## Проверка JWT подписи

Для проверки JWT сервис:

1. Извлекает `vuz_id` из payload.
2. Загружает `universities.public_key`.
3. Проверяет header `alg = RS256`.
4. Проверяет подпись JWT через RSA public key.

Поддерживаемые форматы RSA public key:

- PEM
- base64 DER
- hex DER

Реализация:

- [`pkg/verifyhash/jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/jwt.go)
- [`pkg/verifyhash/signature.go`](/d:/diasoft/pubver/pkg/verifyhash/signature.go)

## Что сервис не делает

Важно понимать текущие границы:

- сервис не проверяет Auth JWT для логина ВУЗа;
- сервис не создает Share Link JWT;
- сервис не читает Kafka;
- сервис не валидирует CSV/JSON загрузки;
- сервис не использует `diploma_hashes.signature` для публичной проверки;
- сервис не обновляет таблицы БД;
- сервис не отзывает дипломы.

## Зависимости от БД

Для своей работы сервис использует:

- `universities`
- `diploma_hashes`
- `diploma_publications`

### `universities`

Используемые поля:

- `id`
- `name`
- `public_key`
- `vuz_code`

Назначение:

- `public_key` — RSA public key для проверки `RS256` подписи QR JWT;
- `vuz_code` — публичный код ВУЗа для endpoint `search`.

### `diploma_hashes`

Используемые поля:

- `hash`
- `vuz_id`
- `diploma_number`
- `status`
- `revoked_at`

Назначение:

- `hash` — основной ключ публичной верификации;
- `status` — финальная истина о действительности диплома.

### `diploma_publications`

Используемые поля:

- `diploma_hash`
- `graduate_year`
- `specialty`

По вашему условию `year` и `specialty` как сущности хранения не менялись и остаются в `diploma_publications`.

## Почему нужна миграция `007`

В исходной схеме не было `vuz_code`, а публичный поиск должен работать именно по нему.

Поэтому миграция [`007_public_verification_support.sql`](/d:/diasoft/pubver/migrations/007_public_verification_support.sql):

- добавляет `universities.vuz_code`;
- создает уникальный индекс по `vuz_code`;
- создает `diploma_publications`, если таблицы еще нет;
- создает индекс по `graduate_year`.

## Переменные окружения

| Переменная | Обязательна | По умолчанию | Назначение |
| --- | --- | --- | --- |
| `DATABASE_URL` | да | нет | строка подключения к PostgreSQL |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `REQUEST_TIMEOUT` | нет | `5s` | timeout на один запрос |
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

В проекте есть:

- [Dockerfile](/d:/diasoft/pubver/Dockerfile)
- [docker-compose.yml](/d:/diasoft/pubver/docker-compose.yml)

Запуск:

```powershell
cd d:\diasoft\pubver
docker compose up --build
```

По умолчанию поднимаются:

- `pubver` на `http://localhost:8080`
- PostgreSQL на `localhost:5432`

## Примеры запросов

### Healthcheck

```powershell
curl http://localhost:8080/healthz
```

### Поиск по номеру диплома

```powershell
curl "http://localhost:8080/api/v1/verify/search?diploma_number=DVS-2024-001234&vuz_code=bmstu"
```

### Проверка по QR JWT

```powershell
curl "http://localhost:8080/api/v1/verify?payload=<jwt>"
```

## Логирование и middleware

HTTP-слой использует middleware для:

- генерации `X-Request-ID`;
- логирования запросов;
- panic recovery;
- timeout на запрос.

Основные файлы:

- [`internal/httpapi/router.go`](/d:/diasoft/pubver/internal/httpapi/router.go)
- [`internal/httpapi/middleware.go`](/d:/diasoft/pubver/internal/httpapi/middleware.go)
- [`internal/httpapi/context.go`](/d:/diasoft/pubver/internal/httpapi/context.go)

## Структура проекта

- [`cmd/pubver/main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) — точка входа, конфиг, пул БД, запуск HTTP-сервера.
- [`internal/config/config.go`](/d:/diasoft/pubver/internal/config/config.go) — чтение env и дефолтов.
- [`internal/domain/model.go`](/d:/diasoft/pubver/internal/domain/model.go) — доменные модели ответов и сущностей.
- [`internal/domain/errors.go`](/d:/diasoft/pubver/internal/domain/errors.go) — доменные ошибки.
- [`internal/httpapi/router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) — маршруты и HTTP-обработка ошибок.
- [`internal/httpapi/middleware.go`](/d:/diasoft/pubver/internal/httpapi/middleware.go) — request id, логирование, recovery.
- [`internal/httpapi/context.go`](/d:/diasoft/pubver/internal/httpapi/context.go) — прокидывание request id через context.
- [`internal/service/verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) — логика `verify` и `search`.
- [`internal/repository/repository.go`](/d:/diasoft/pubver/internal/repository/repository.go) — интерфейс репозитория.
- [`internal/repository/postgres/verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) — SQL-запросы к PostgreSQL.
- [`pkg/verifyhash/jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/jwt.go) — разбор JWT header/payload.
- [`pkg/verifyhash/hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) — сбор raw-строки и вычисление `SHA-256`.
- [`pkg/verifyhash/signature.go`](/d:/diasoft/pubver/pkg/verifyhash/signature.go) — проверка `RS256` подписи JWT.
- [`openapi/public-verification.yaml`](/d:/diasoft/pubver/openapi/public-verification.yaml) — OpenAPI-контракт.
- [`docs/architecture.md`](/d:/diasoft/pubver/docs/architecture.md) — архитектурное описание.

## Тестирование

В проекте есть unit-тесты для:

- извлечения claims из JWT;
- вычисления хеша;
- проверки `RS256`;
- бизнес-логики `verify` и `search`.

Запуск:

```powershell
cd d:\diasoft\pubver
go test ./...
```

## Рекомендации для продакшена

- создать отдельного read-only пользователя PostgreSQL;
- не логировать raw JWT и query string целиком;
- поставить сервис за rate limiting;
- читать из replica, если ожидается большая публичная нагрузка;
- добавить явную проверку `iss`, `aud`, `exp`, если они станут обязательными в QR JWT;
- добавить аудит неуспешных попыток проверки;
- добавить метрики по числу запросов `verify/search` и ошибкам БД.

## Ближайшие логичные доработки

- добавить интеграционные тесты с PostgreSQL;
- зафиксировать единый контракт QR JWT и Crypto Engine в одном документе;
- добавить миграции `001..006` в этот репозиторий, если сервис будет разворачиваться автономно;
- при необходимости поддержать `kid` и несколько RSA ключей на один ВУЗ.
