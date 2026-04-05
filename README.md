# Diasoft Diploma Registry

Единая платформа для загрузки, криптографической обработки, публикации и проверки дипломов.

Проект состоит из нескольких сервисов:

- `frontend` - React UI для администратора, вуза, студента и публичной проверки
- `gateway-service` - основной API для админа, вуза, батчей, API-ключей и студенческих share-ссылок
- `cryptography-engine-rs` - асинхронный криптографический воркер на Rust
- `pubver` - публичная проверка дипломов по QR/JWT и по `vuz_code + diploma_number`
- `postgres` - основная БД
- `kafka` - очередь задач и событий
- `redis` - rate limit и cache
- `kafka-ui` - интерфейс для просмотра топиков
- `db-migrate` - сервис применения SQL-миграций

`verify-service` как отдельный сервис больше не используется. Публичная проверка реализована в `pubver`.

## Что решает система

- вуз загружает дипломы батчами в JSON или CSV
- система вычисляет детерминированный хэш диплома
- данные диплома шифруются внутри QR payload
- QR/JWT подписывается приватным ключом вуза
- студент может найти диплом, выпустить share-ссылку и заново сгенерировать QR
- работодатель или внешний пользователь может проверить диплом через публичный сервис
- администратор платформы управляет вузами и следит за метриками

## Роли

- `admin` - подтверждает и блокирует вузы, смотрит статистику, управляет платформой
- `university` - загружает дипломы, получает API-ключи, настраивает signing key
- `student` - без входа ищет свой диплом, генерирует share-link и QR
- `public verifier` - проверяет диплом по QR или по коду вуза и номеру диплома

## Архитектура

### Контур запросов

1. Пользователь открывает `frontend`.
2. `frontend` проксирует:
   - `/api/*` -> `gateway-service`
   - `/verify-api/*` -> `pubver`
3. `gateway-service` работает с PostgreSQL, Redis и Kafka.
4. `cryptography-engine-rs` читает задания из Kafka, работает с PostgreSQL и публикует результат обратно в Kafka.
5. `pubver` читает PostgreSQL, использует Redis для rate limit/cache и пишет аналитические события в Kafka.

### Поток загрузки дипломов

1. Вуз отправляет `POST /api/v1/diplomas/upload`.
2. `gateway-service`:
   - валидирует строки
   - создаёт запись батча в `batches`
   - сохраняет исходные данные каждой строки в `batch_record_payloads`
   - шифрует payload перед записью в БД
   - публикует задания в Kafka topic `diplomas.raw_tasks`
3. `cryptography-engine-rs`:
   - забирает задачу из `diplomas.raw_tasks`
   - загружает приватный ключ вуза из `university_signing_keys`
   - расшифровывает приватный ключ
   - вычисляет соль и хэш диплома
   - формирует QR payload
   - подписывает JWT алгоритмом `Ed25519`
   - сохраняет итог в `diploma_hashes` и `batch_results`
   - обновляет прогресс батча в `batches`
   - публикует результат в `diplomas.processing_results`
4. `gateway-service` читает result-topic и умеет применять результат повторно для совместимости.
5. Вуз видит прогресс батча и может скачать Excel с итогами.

### Поток публичной проверки

#### Вариант 1. Проверка по QR/JWT

1. Пользователь сканирует QR диплома.
2. Браузер открывает страницу `frontend` `/verify?payload=...`.
3. `frontend` обращается в `pubver`.
4. `pubver`:
   - без доверия читает `vuz_id` из JWT
   - получает `universities.public_key`
   - проверяет подпись JWT
   - расшифровывает `enc`
   - пересчитывает хэш
   - сравнивает его с `sub` и `diploma_hash`
   - сверяет итог с `diploma_hashes`

#### Вариант 2. Проверка по коду вуза и номеру диплома

1. Пользователь вводит `vuz_code` и `diploma_number`.
2. `pubver` ищет запись в `diploma_hashes` и `universities`.
3. Метаданные диплома подтягиваются из `batch_results.qr_payload` после расшифровки `enc`.

### Поток студенческого портала

1. Студент находит диплом через `gateway-service` по `diploma_number + full_name`.
2. `gateway-service` выдаёт найденные записи.
3. Студент может:
   - создать share-link
   - сгенерировать новый QR для работодателя
4. Share-link хранится в `share_links`, а payload ссылки подписывается HMAC JWT.

## Сервисы и их ответственность

### `frontend`

- React + Ant Design + Tailwind
- публичные страницы:
  - главная
  - документация
  - проверка диплома
  - портал студента
  - страница проверки после QR
- приватные страницы:
  - кабинет администратора
  - кабинет вуза

### `gateway-service`

Отвечает за:

- регистрацию и логин
- bootstrap admin и demo-вуз
- кабинет вуза
- кабинет администратора
- API-ключи для ERP
- приём JSON/CSV батчей
- выдачу статуса батча
- генерацию Excel
- revoke диплома
- student search, share-link и QR на share-link
- сбор verification analytics из Kafka

### `cryptography-engine-rs`

Отвечает за:

- расчёт соли
- расчёт хэша диплома
- сборку и шифрование QR payload
- подпись JWT Ed25519
- запись результатов криптообработки в PostgreSQL
- публикацию результата в Kafka

### `pubver`

Отвечает за:

- `GET /api/v1/verify`
- `GET /api/v1/verify/search`
- rate limiting публичных ручек
- cache результатов проверки
- публикацию verification analytics в Kafka

## PostgreSQL: таблицы

### Основные таблицы

- `platform_admins`
  Назначение: учётные записи администраторов платформы.

- `universities`
  Назначение: карточка вуза.
  Хранит: `name`, `vuz_code`, `inn`, `ogrn`, `email`, `password_hash`, `public_key`, `status`.

- `api_keys`
  Назначение: ERP-доступ от имени вуза.

- `university_signing_keys`
  Назначение: приватный ключ вуза для подписи QR/JWT.
  Хранит: `encrypted_private_key`, `key_algorithm`, `encryption_algorithm`, `public_key_fingerprint`.

- `batches`
  Назначение: батч загрузки дипломов.
  Хранит: `status`, `total_records`, `processed_records`, `created_at`, `completed_at`.

- `batch_record_payloads`
  Назначение: исходные данные строк батча.
  Важно: хранит не plaintext, а `encrypted_payload`.

- `batch_results`
  Назначение: результат обработки каждой строки батча.
  Хранит: `diploma_hash`, `qr_payload`, `status`, `error`.

- `diploma_hashes`
  Назначение: реестр дипломов.
  Хранит: `hash`, `vuz_id`, `diploma_number`, `status`, `revoked_at`.

- `share_links`
  Назначение: публичные share-ссылки студента.

- `verification_events`
  Назначение: аналитика публичных проверок.

### Служебные таблицы миграций

- `file_schema_migrations`
  Создаётся `deploy/migrate.sh`, хранит уже применённые SQL-файлы.

- `go_schema_migrations`
  Может встречаться как legacy-таблица от более ранних экспериментов с `golang-migrate`.
  Для текущего деплоя она не требуется.

### Удалённые legacy-таблицы

- `batch_records` - удалена
- `batch_record_attributes` - удалена

Текущая модель не хранит открытые персональные данные в этих старых таблицах.

## Что хранится в открытом виде, а что шифруется

### Шифруется в БД

- `batch_record_payloads.encrypted_payload`
  Внутри: JSON с `full_name`, `diploma_number`, `specialty`, `degree`, `faculty`, `year`

- `university_signing_keys.encrypted_private_key`
  Внутри: PEM приватного ключа вуза

### Хранится в подписанном и зашифрованном QR/JWT

Внешний JWT содержит:

```json
{
  "sub": "<diploma_hash>",
  "diploma_hash": "<diploma_hash>",
  "vuz_id": "<uuid>",
  "enc": "<base64(nonce|ciphertext|tag)>",
  "iat": 1710000000
}
```

Поле `enc` после расшифровки даёт:

```json
{
  "full_name": "...",
  "diploma_number": "...",
  "specialty": "...",
  "degree": "...",
  "faculty": "...",
  "year": 2026,
  "salt": "..."
}
```

### Хранится в открытом виде

- `diploma_hashes.diploma_number`
  Нужен для поиска и проверки по номеру диплома.

- служебные статусы, идентификаторы вузов, коды вузов, timestamps

## Криптография и безопасность

### Хэш диплома

Каноническая строка:

```text
diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt
```

Хэш:

```text
SHA-256(canonical_string)
```

### Соль

Соль вычисляется детерминированно:

```text
salt = SHA-256("diasoft|vuz_id|diploma_number")
```

Это даёт:

- повторяемый хэш для одного и того же диплома
- защиту от простого перебора одинаковых данных между разными вузами

### Подпись QR/JWT

- алгоритм: `Ed25519`
- JWT header: `alg = EdDSA`
- приватный ключ хранится у вуза в зашифрованном виде
- публичный ключ хранится в `universities.public_key`

### Шифрование payload

- алгоритм: `AES-256-GCM`
- используется для:
  - `batch_record_payloads.encrypted_payload`
  - `university_signing_keys.encrypted_private_key`
  - `enc` внутри QR/JWT

### Токены приложения

- access token: `HS256`
- share-link token: `HS256`

## Kafka

### Топики

- `diplomas.raw_tasks`
  Исходные задания на криптообработку дипломов.

- `diplomas.processing_results`
  Результаты обработки строк батча.

- `verification.events`
  События публичной проверки и аналитики.

### Почему Kafka здесь нужна

- отделяет загрузку батча от тяжёлой криптообработки
- даёт асинхронный pipeline
- упрощает масштабирование воркера
- позволяет отдельно собирать verification analytics

## Redis

Используется для:

- rate limit на `gateway-service`
- rate limit на `pubver`
- cache административных и публичных read-ручек

## HTTP API

### Основные ручки `gateway-service`

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/admin/universities`
- `POST /api/v1/admin/universities/{id}/approve`
- `GET /api/v1/admin/stats`
- `GET /api/v1/vuz/profile`
- `GET /api/v1/vuz/batches`
- `POST /api/v1/vuz/api-keys`
- `GET /api/v1/vuz/api-keys`
- `PUT /api/v1/vuz/signing-key`
- `GET /api/v1/vuz/signing-key`
- `POST /api/v1/diplomas/upload`
- `GET /api/v1/diplomas/batches/{batch_id}`
- `GET /api/v1/diplomas/batches/{batch_id}/download`
- `PATCH /api/v1/diplomas/{diploma_hash}/revoke`
- `GET /api/v1/student/search`
- `POST /api/v1/student/share`
- `GET /api/v1/student/qr`
- `GET /api/v1/student/share/{token}`

### Основные ручки `pubver`

- `GET /api/v1/verify?payload=<jwt>`
- `GET /api/v1/verify/search?diploma_number=<number>&vuz_code=<code>`

## Docker Compose

Корневой стек поднимается из:

- [`docker-compose.yml`](./docker-compose.yml)

Сервисы и внешние порты:

- `frontend` -> `http://localhost`
- `gateway-service` -> `http://localhost:8080`
- `pubver` -> `http://localhost:8082`
- `kafka-ui` -> `http://localhost:8081`
- `postgres` -> `localhost:5432`
- `kafka` -> `localhost:9092`
- `redis` -> `localhost:6379`

## Первый запуск

```bash
cp .env.example .env
docker compose up -d --build
```

Для сервера обязательно поправить в `.env`:

- `PUBLIC_BASE_URL=https://ваш-домен-или-ip`
- `KAFKA_EXTERNAL_HOST=<серверный-ip-или-hostname>` если нужен внешний доступ к Kafka

Порядок старта:

1. `postgres`
2. `db-migrate`
3. `kafka`
4. `kafka-init`
5. `redis`
6. `gateway-service`
7. `cryptography-engine-rs`
8. `pubver`
9. `frontend`

## Как работает `db-migrate`

Для production/deploy используется file-based мигратор:

- Dockerfile: [`deploy/db-migrate.Dockerfile`](./deploy/db-migrate.Dockerfile)
- script: [`deploy/migrate.sh`](./deploy/migrate.sh)

Это сделано специально, потому что текущий набор SQL-файлов является рабочим, но не полностью совместим с `golang-migrate` по version-based правилам.

## Troubleshooting

### `db-migrate` завершился с `exit 1`

Проверить:

1. совпадает ли пароль Postgres в `docker-compose.yml`
2. не остался ли старый volume, созданный на сломанной конфигурации
3. доступен ли `DATABASE_URL` внутри `db-migrate`
4. нет ли legacy-таблиц миграций от предыдущего способа запуска

Если БД не нужна и можно начать с нуля:

```bash
docker compose down -v
docker compose up -d --build
```

Если volume нужно сохранить, а раньше Postgres был поднят с другим паролем, надо отдельно сменить пароль пользователя в уже существующей БД.

Текущий `deploy/migrate.sh` умеет автоматически подхватывать:

- `schema_migrations` с колонкой `version`
- `schema_migrations` с колонкой `filename`
- `go_schema_migrations`
- уже существующую схему БД, даже если трекинг SQL-файлов ещё не был создан

### `404` при логине demo-пользователей

Было две причины:

- frontend nginx не проксировал `/api/` в `gateway-service`
- demo admin и demo university не были гарантированно созданы при старте

Текущее состояние:

- `frontend/nginx.conf` проксирует `/api/` и `/verify-api/`
- `gateway-service` на старте делает bootstrap admin и demo university

### Батч завис в `pending`

Проверить:

- есть ли сообщения в `diplomas.raw_tasks`
- читает ли их `cryptography-engine-rs`
- обновляется ли `batches.processed_records`
- есть ли строки в `batch_results`

Текущая реализация воркера пишет результат в PostgreSQL напрямую и только после этого публикует сообщение в result-topic.

## Demo-данные

### Тестовые пользователи

- admin: `admin@platform.local / Admin12345!`
- university: `demo.vuz@platform.local / University123!`

### Demo-вуз

- `vuz_code = DEMO2026`

### Demo private key

- [`deploy/demo/demo_university_ed25519_private_key.pem`](./deploy/demo/demo_university_ed25519_private_key.pem)

## Что говорить на защите

Короткая версия:

1. `gateway-service` принимает батчи и управляет платформой.
2. `cryptography-engine-rs` отвечает за хэширование, шифрование и подпись QR/JWT.
3. `pubver` изолирует публичную проверку от внутреннего кабинета вуза.
4. В БД не хранится plaintext-таблица с полными данными дипломов; исходные записи батча лежат в зашифрованном виде.
5. Проверка диплома не доверяет payload вслепую: подпись JWT проверяется, `enc` расшифровывается, хэш пересчитывается заново и сверяется с реестром.
6. Kafka отделяет тяжёлую криптообработку от пользовательского API и позволяет масштабировать pipeline.

## Дополнительная документация

- [`gateway-service/PROJECT_DOCUMENTATION.md`](./gateway-service/PROJECT_DOCUMENTATION.md)
- [`gateway-service/README.md`](./gateway-service/README.md)
- [`pubver/README.md`](./pubver/README.md)
