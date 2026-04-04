# Что `pubver` нужно от Crypto Service

Этот файл описывает, что именно должен делать `Crypto & Processing Engine`, чтобы `Public Verification API` мог корректно проверять дипломы по QR.

## Коротко

Для работы `pubver` от `Crypto Service` нужны 4 вещи:

1. Корректный `Ed25519`-подписанный QR JWT с `alg: "EdDSA"`.
2. Корректный `SHA-256` по фиксированному контракту.
3. Согласованный `vuz_id`, который совпадает с `universities.id` в БД.
4. Результат обработки, который основной API сохранит в `diploma_hashes`.

Без этого `GET /api/v1/verify?payload=<jwt>` не сможет подтвердить диплом.

## Что делает `pubver`

`pubver` при запросе `GET /api/v1/verify?payload=<jwt>`:

1. Без доверия читает из JWT только `vuz_id`.
2. По `vuz_id` загружает `universities.public_key`.
3. Проверяет JWT как `EdDSA`, то есть подпись `Ed25519`.
4. Из валидного JWT достает данные диплома.
5. Пересчитывает:

```text
SHA-256(full_name|diploma_number|specialty|degree|faculty|year|vuz_id|salt)
```

6. Сверяет результат с `sub` и `diploma_hash`, если они есть.
7. Ищет пересчитанный хеш в `diploma_hashes`.
8. Возвращает `active`, `revoked` или `not_found`.

Из этого следует, что `Crypto Service` обязан выпустить JWT и хеш так, чтобы оба шага проверки совпали:

- криптографическая подпись JWT
- пересчитанный хеш диплома

## Что обязательно должен делать `Crypto Service`

### 1. Генерировать `salt`

Для каждого диплома нужна отдельная соль.

Требование:

- `salt` должен быть непустой строкой
- `salt` должен входить и в QR JWT, и в расчет хеша

Пример:

```text
abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789
```

## 2. Считать хеш строго по зафиксированному контракту

Формула должна быть ровно такой:

```text
raw = full_name|diploma_number|specialty|degree|faculty|year|vuz_id|salt
hash = SHA-256(raw)
```

Важно:

- тот же порядок полей
- тот же разделитель `|`
- никаких лишних пробелов, JSON-канонизаций и других форматов
- результат должен храниться и передаваться как lower-case hex string

Если `Crypto Service` посчитает хеш хоть немного иначе, `pubver` не найдет запись в `diploma_hashes`.

## 3. Формировать QR JWT в плоском виде

`pubver` ожидает не вложенный `student`, а плоский payload.

Ожидаемый header:

```json
{
  "alg": "EdDSA",
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
  "degree": "Бакалавр",
  "faculty": "ФКН",
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
- `degree`
- `faculty`
- `year`
- `salt`

Рекомендуемые claims:

- `sub`
- `diploma_hash`
- `iat`

`sub` и `diploma_hash` лучше заполнять одинаковым значением, равным пересчитанному хешу.

`degree` и `faculty` должны передаваться в JWT как обычные строковые claims. `pubver` валидирует их наличие и использует их в формуле хеша, но пока не возвращает в публичном ответе.

## 4. Подписывать QR JWT ключом `Ed25519`

`pubver` больше не использует `RS256`.

Требование:

- JWT должен быть подписан как `EdDSA`
- фактический алгоритм подписи - `Ed25519`
- private key у `Crypto Service`
- соответствующий public key должен лежать в `universities.public_key`

Если public key в БД не соответствует private key, которым подписан JWT, `pubver` вернет:

- `400 invalid verification payload`

## Что должно совпадать между `Crypto Service` и БД

Чтобы верификация сработала, после обработки диплома в БД должны оказаться:

### `universities`

- `id` = тот же `vuz_id`, который кладется в JWT
- `public_key` = `Ed25519` public key для проверки JWT
- `name`
- `vuz_code`

### `diploma_hashes`

- `hash` = результат `SHA-256`
- `vuz_id`
- `diploma_number`
- `status`
- `revoked_at`, если диплом отозван

Сам `Crypto Service` может не писать это напрямую, но он должен отдать результат в таком виде, чтобы основной API смог сохранить его без преобразований, меняющих контракт.

## Минимальный результат, который должен вернуться из `Crypto Service`

Если ориентироваться на текущую связку сервисов, на выходе нужен как минимум такой объект:

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

Ключевые поля для `pubver` здесь:

- `vuz_id`
- `diploma_hash`
- `qr_payload`

## Что именно проверяет `pubver`

`pubver` считает диплом валидным только если одновременно выполняются все условия:

1. JWT успешно проходит `Ed25519`-проверку.
2. Из JWT можно собрать все обязательные claims.
3. Пересчитанный `SHA-256` совпадает с `sub` и `diploma_hash`, если они переданы.
4. Такой хеш найден в `diploma_hashes`.
5. У записи в реестре статус `active`.

Если хотя бы один из пунктов не выполняется, пользователь не увидит `valid: true`.

## Что сейчас не требуется от `Crypto Service`

С точки зрения текущего `pubver` не требуется:

- `RS256`
- `diploma_hashes.signature`
- отдельная подпись записи диплома
- хранение `year` и `specialty` в PostgreSQL для публичной выдачи
- `share_links`

Но важно:

- `year` и `specialty` не хранятся пока в БД для `search`
- при этом они обязательны для QR JWT и обязательны для расчета хеша

То есть убирать их из payload нельзя.

## Частые причины, почему `pubver` отклонит токен

### `400 invalid verification payload`

Причины:

- JWT поврежден
- неверный `alg`
- подпись `Ed25519` не проходит
- отсутствует `vuz_id`
- `universities.public_key` не найден
- не хватает `student_name`, `diploma_number`, `specialty`, `degree`, `faculty`, `year` или `salt`
- `sub` не совпадает с пересчитанным хешем
- `diploma_hash` не совпадает с пересчитанным хешем

### `200 { valid: false, status: "not_found" }`

Причина:

- JWT валиден
- хеш пересчитался
- но такого `hash` нет в `diploma_hashes`

### `200 { valid: false, status: "revoked" }`

Причина:

- запись в реестре найдена
- но `status = revoked`

## Итоговый контракт для команды Crypto

Команде `Crypto Service` нужно обеспечить следующее:

1. Выпускать QR JWT с `alg: "EdDSA"`.
2. Подписывать JWT приватным ключом `Ed25519`.
3. Использовать тот же `vuz_id`, который есть в `universities.id`.
4. Класть в JWT обязательные claims:
   - `vuz_id`
   - `diploma_number`
   - `student_name` или `full_name`
   - `specialty`
   - `degree`
   - `faculty`
   - `year`
   - `salt`
5. Считать хеш строго по формуле:

```text
SHA-256(full_name|diploma_number|specialty|degree|faculty|year|vuz_id|salt)
```

6. Отдавать этот хеш как `sub` и `diploma_hash`.
7. Передавать результат обработки так, чтобы основной API сохранил:
   - `universities.public_key`
   - `diploma_hashes.hash`
   - `diploma_hashes.vuz_id`
   - `diploma_hashes.diploma_number`
   - `diploma_hashes.status`

Если все это соблюдено, `pubver` сможет проверять QR стабильно и без дополнительных преобразований.
