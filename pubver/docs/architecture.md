# Архитектура Public Verification API

## Роль сервиса

`pubver` - внешний read-only сервис для публичной проверки диплома после сканирования QR-кода и для ограниченного поиска по номеру диплома.

Сервис:

- не пишет в реестр дипломов
- не работает с Kafka
- не обрабатывает batch upload
- не расшифровывает `encrypted_payload`
- не меняет статус диплома
- проверяет `EdDSA` подпись QR JWT
- пересчитывает хеш и сверяет его с реестром

Сервис работает только через реальную `PostgreSQL`.

## Поток `GET /api/v1/verify`

1. Клиент открывает ссылку `/api/v1/verify?payload=<jwt>`.
2. Сервис извлекает `vuz_id` из payload без доверия к данным.
3. По `vuz_id` находит `universities.public_key`.
4. Проверяет `EdDSA` подпись JWT через `Ed25519`.
5. Из валидного payload читает `sub`, `diploma_hash`, `vuz_id`, `diploma_number`, `student_name`, `specialty`, `degree`, `faculty`, `year`, `salt`, `iat`.
6. Собирает строку:

```text
student_name|diploma_number|specialty|degree|faculty|year|vuz_id|salt
```

7. Считает `SHA-256`.
8. Сверяет результат с обязательными `sub` и `diploma_hash`.
9. Ищет хеш в `diploma_hashes`.
10. Возвращает `active`, `revoked` или `not_found`.

## Поток `GET /api/v1/verify/search`

1. Клиент передает `diploma_number` и `vuz_code`.
2. Сервис ищет запись по `universities.vuz_code + diploma_hashes.diploma_number`.
3. Возвращает публичный статус и дополнительные поля `year` и `specialty`.
4. Пока схема БД для этих полей не утверждена, сервис отдает их как `null`-заглушки.

## Временное состояние `year` и `specialty`

Поля сохранены:

- в доменных моделях
- в JSON-ответах
- в OpenAPI

Но пока не хранятся в PostgreSQL и не участвуют в SQL-запросах публичного сервиса.

Это позволяет:

- сохранить стабильный API-контракт
- не добавлять в БД временную схему
- подключить реальные данные позже без смены ответа

`degree` и `faculty` приходят в QR JWT, валидируются на этапе разбора claims и участвуют в формуле хеша, но пока не отдаются наружу публичным API.

## Почему нужен `vuz_code`

В исходной схеме `universities` не было публичного идентификатора для поиска. Поэтому миграция добавляет `vuz_code`.

Под `vuz_code` здесь понимается код формата `001X7276`, то есть код по сводному реестру, а не алиас вроде `bmstu`.

## Границы ответственности

### Main API / Gateway

- принимает загрузки дипломов
- создает batch-задачи
- пишет в Kafka
- сохраняет результаты обработки

### Crypto Engine

- генерирует `salt`
- считает `SHA-256` по согласованному контракту
- формирует QR JWT
- подписывает QR JWT приватным ключом `Ed25519` ВУЗа

### Public Verification API

- извлекает `vuz_id` из JWT
- находит `Ed25519` public key ВУЗа
- проверяет `EdDSA` подпись JWT
- пересчитывает хеш
- ищет диплом в БД
- возвращает публичный результат

## Что сервис сознательно не делает

- не использует `diploma_hashes.signature` в публичной верификации
- не определяет валидность диплома только по JWT без lookup в БД
- не хранит пока `year` и `specialty` в PostgreSQL

## Структура кода

- [`main.go`](/d:/diasoft/pubver/cmd/pubver/main.go) - запуск приложения
- [`router.go`](/d:/diasoft/pubver/internal/httpapi/router.go) - HTTP endpoints
- [`verification_service.go`](/d:/diasoft/pubver/internal/service/verification_service.go) - бизнес-логика
- [`verification_repository.go`](/d:/diasoft/pubver/internal/repository/postgres/verification_repository.go) - SQL к PostgreSQL
- [`hash.go`](/d:/diasoft/pubver/pkg/verifyhash/hash.go) - хеширование
- [`qr_jwt.go`](/d:/diasoft/pubver/pkg/verifyhash/qr_jwt.go) - разбор QR JWT
- [`ed25519.go`](/d:/diasoft/pubver/pkg/verifyhash/ed25519.go) - проверка `Ed25519`
