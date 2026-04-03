# Архитектура Public Verification API

## Роль сервиса

`pubver` — внешний read-only сервис для публичной проверки диплома после сканирования QR-кода и для ограниченного поиска по номеру диплома.

Сервис:

- не пишет в реестр дипломов;
- не работает с `encrypted_payload`;
- не читает Kafka;
- не подписывает данные;
- проверяет `RS256` подпись QR JWT;
- пересчитывает хеш и ищет его в БД;
- возвращает публичный статус диплома.

## Поток `GET /api/v1/verify`

1. Клиент сканирует QR и открывает ссылку `/api/v1/verify?payload=<jwt>`.
2. Сервис декодирует payload без доверия к данным.
3. Из payload извлекается `vuz_id`.
4. По `vuz_id` сервис находит `universities.public_key`.
5. Выполняется проверка `RS256` подписи самого JWT.
6. Из валидного payload извлекаются `student_name`, `diploma_number`, `specialty`, `year`, `vuz_id`, `salt`.
7. Собирается строка:

```text
full_name|diploma_number|specialty|year|vuz_id|salt
```

8. Считается `SHA-256`.
9. Проверяется совпадение с `sub` и `diploma_hash`, если они есть в JWT.
10. Выполняется поиск по `diploma_hashes.hash`.
11. Возвращается `active`, `revoked` или `not_found`.

## Поток `GET /api/v1/verify/search`

1. Клиент передает `diploma_number` и `vuz_code`.
2. Сервис ищет запись по `universities.vuz_code + diploma_hashes.diploma_number`.
3. Возвращает публичный статус и дополнительные безопасные поля из `diploma_publications`.

## Почему нужен `vuz_code`

В исходной схеме `universities` не было публичного короткого идентификатора для поиска. Поэтому в миграции добавляется `vuz_code`.

## Что осталось без изменений

- `year` и `specialty` как сущности хранения не менялись;
- `diploma_publications` остается источником этих полей для публичного ответа;
- `diploma_hashes.status` остается финальной истиной о действительности диплома.

## Границы ответственности

### Main API / Gateway

- принимает загрузки дипломов;
- создает батчи;
- публикует задачи в Kafka;
- сохраняет результаты обработки.

### Crypto Engine

- генерирует `salt`;
- вычисляет `SHA-256` по согласованному контракту;
- формирует `qr_payload`;
- подписывает QR JWT приватным RSA-ключом ВУЗа.

### Public Verification API

- извлекает `vuz_id` из JWT;
- находит RSA public key ВУЗа;
- проверяет `RS256` подпись JWT;
- пересчитывает хеш;
- ищет диплом в БД;
- возвращает публичный результат.

## Что сервис сознательно не делает

- не проверяет отдельную подпись записи диплома;
- не использует `diploma_hashes.signature` в публичной верификации;
- не определяет валидность диплома только по JWT без lookup в БД.

## Структура кода

- [`cmd/pubver/main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) — запуск приложения и graceful shutdown.
- [`internal/httpapi/router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) — HTTP endpoints.
- [`internal/service/verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) — бизнес-логика проверки и поиска.
- [`internal/repository/postgres/verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) — SQL-запросы к PostgreSQL.
- [`pkg/verifyhash/hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) — контракт хеширования.
- [`pkg/verifyhash/jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/jwt.go) — разбор JWT header и payload.
- [`pkg/verifyhash/signature.go`](/d:/diasoft/pubver/pkg/verifyhash/signature.go) — проверка `RS256` подписи JWT.
