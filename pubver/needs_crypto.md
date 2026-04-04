# Что `pubver` нужно от Crypto Service

## Коротко

Чтобы `pubver` корректно проверял дипломы, `Crypto Service` должен:

1. Выпускать JWT с `alg: "EdDSA"`.
2. Подписывать JWT приватным ключом `Ed25519`.
3. Хранить персональные данные диплома внутри claim `enc`, зашифрованного `A256GCM`.
4. Считать хеш по формуле:

```text
SHA-256(diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt)
```

5. Класть этот хеш одинаково в `sub` и `diploma_hash`.

## Верхний JWT payload

```json
{
  "sub": "sha256-hash",
  "diploma_hash": "sha256-hash",
  "vuz_id": "550e8400-e29b-41d4-a716-446655440000",
  "enc": "<base64(nonce|ciphertext|tag)>",
  "iat": 1710000000
}
```

Обязательные поля:

- `sub`
- `diploma_hash`
- `vuz_id`
- `enc`
- `iat`

## Что должно лежать в `enc`

После расшифровки `enc` сервис ожидает JSON:

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

Обязательные поля:

- `full_name`
- `diploma_number`
- `specialty`
- `degree`
- `faculty`
- `year`
- `salt`

## Формат `enc`

`pubver` ожидает, что `enc`:

- зашифрован алгоритмом `A256GCM`
- закодирован в base64
- содержит байты в формате:

```text
nonce(12 bytes) | ciphertext+tag
```

## Что проверяет `pubver`

1. Из JWT достается `vuz_id`.
2. По `vuz_id` грузится `universities.public_key`.
3. JWT проверяется через `Ed25519`.
4. `enc` расшифровывается через `A256GCM`.
5. Из расшифрованного JSON собирается строка для хеша.
6. Пересчитывается `SHA-256`.
7. Хеш сверяется с `sub` и `diploma_hash`.
8. Хеш ищется в `diploma_hashes`.
9. Возвращается `active`, `revoked` или `not_found`.

## Что должно совпасть с БД

### `universities`

- `id` = тот же `vuz_id`, который приходит в JWT
- `public_key` = `Ed25519` public key для проверки JWT
- `name`
- `vuz_code`

### `diploma_hashes`

- `hash` = тот же `SHA-256`
- `vuz_id`
- `diploma_number`
- `status`

## Минимальный результат от Crypto Service

```json
{
  "batch_id": "uuid",
  "vuz_id": "uuid",
  "record_index": 42,
  "diploma_hash": "sha256hex",
  "qr_payload": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "status": "ok",
  "error": null
}
```
