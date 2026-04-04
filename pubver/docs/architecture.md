# Архитектура Public Verification API

## Роль сервиса

`pubver` - внешний read-only сервис публичной проверки дипломов.

Он:

- не пишет в реестр
- не читает Kafka
- не расшифровывает `encrypted_payload` из `batch_results`
- не меняет статус диплома
- проверяет `EdDSA` подпись JWT
- расшифровывает claim `enc` через `A256GCM`
- пересчитывает `SHA-256`
- сверяет хеш с реестром

## Поток `/api/v1/verify`

1. Клиент открывает `/api/v1/verify?payload=<jwt>`.
2. Сервис без доверия читает `vuz_id`.
3. По `vuz_id` грузит `universities.public_key`.
4. Проверяет подпись JWT через `Ed25519`.
5. Из верхнего payload читает:
   - `sub`
   - `diploma_hash`
   - `vuz_id`
   - `enc`
   - `iat`
6. Расшифровывает `enc` через `A256GCM`.
7. Получает JSON:
   - `full_name`
   - `diploma_number`
   - `specialty`
   - `degree`
   - `faculty`
   - `year`
   - `salt`
8. Считает:

```text
SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt)
```

9. Сверяет результат с `sub` и `diploma_hash`.
10. Ищет хеш в `diploma_hashes`.
11. Возвращает `active`, `revoked` или `not_found`.

## Поток `/api/v1/verify/search`

1. Клиент передает `diploma_number` и `vuz_code`.
2. Сервис ищет запись по `universities.vuz_code + diploma_hashes.diploma_number`.
3. Для публичных атрибутов подтягивает `year`, `specialty`, `degree`, `faculty`
   из `batch_record_attributes` через `batch_results`.
4. Возвращает статус и публичные поля.

## Используемые таблицы

- `universities`
- `diploma_hashes`
- `batch_results`
- `batch_record_attributes`

## Криптографический контракт

- подпись JWT: `Ed25519` (`alg = EdDSA`)
- шифрование `enc`: `A256GCM`
- формат `enc`: `base64(nonce|ciphertext|tag)`
- хеш диплома: `SHA-256`

## Границы ответственности

### Crypto Engine

- генерирует `salt`
- формирует внутренний JSON диплома
- шифрует его в `enc` через `A256GCM`
- подписывает JWT приватным ключом `Ed25519`
- отдает `diploma_hash`, `vuz_id`, `qr_payload`

### Public Verification API

- проверяет подпись JWT
- расшифровывает `enc`
- пересчитывает хеш
- сверяет хеш с реестром
- возвращает публичный результат
