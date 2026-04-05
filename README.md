# Diasoft Diploma Registry

Единая платформа для загрузки дипломов, криптографической обработки, публикации QR/JWT и публичной проверки подлинности дипломов.

Система состоит из нескольких сервисов и разворачивается одним `docker compose` стеком. Основной сценарий такой:

1. вуз загружает JSON/CSV батч дипломов в `gateway-service`;
2. `gateway-service` пишет батч в PostgreSQL и публикует задачи в Kafka;
3. `cryptography-engine-rs` забирает задачи из Kafka, рассчитывает хэш, шифрует payload, подписывает JWT и сохраняет результат;
4. `frontend` показывает вузу прогресс батча и позволяет скачать Excel с итогом;
5. `pubver` выполняет публичную проверку дипломов по QR/JWT или по `vuz_code + diploma_number`.

## Состав стека

- `frontend` — React UI, публичные страницы, кабинет вуза, кабинет администратора
- `gateway-service` — основной backend для auth, кабинетов, загрузки батчей, API-ключей и student portal
- `cryptography-engine-rs` — асинхронный криптографический воркер на Rust
- `pubver` — публичный сервис проверки дипломов
- `postgres` — основная БД
- `kafka` — очередь задач и событий
- `redis` — rate limit и cache
- `kafka-ui` — просмотр Kafka topic-ов
- `db-migrate` — контейнер применения SQL-миграций

## Архитектура и взаимодействие сервисов

### 1. Frontend

`frontend` отдаётся через nginx и проксирует:

- `/api/*` -> `gateway-service:8080`
- `/verify-api/*` -> `pubver:8080`

Из браузера прямых запросов к Kafka/PostgreSQL нет.

### 2. Gateway Service

`gateway-service` отвечает за:

- регистрацию и логин
- bootstrap администратора
- регистрацию и управление вузами
- личный кабинет вуза
- загрузку дипломов JSON/CSV
- хранение батчей и результатов
- выдачу Excel-отчёта
- API keys для ERP-интеграции
- student portal: поиск диплома, share-link, повторная генерация QR
- приём verification analytics из Kafka

Использует:

- PostgreSQL как основную БД
- Kafka для `diplomas.raw_tasks`, `diplomas.processing_results`, `verification.events`
- Redis для rate limit и cache

### 3. Cryptography Engine

`cryptography-engine-rs` не поднимает HTTP API. Он работает только через Kafka и PostgreSQL:

- читает `diplomas.raw_tasks`
- получает приватный ключ вуза из `university_signing_keys`
- расшифровывает приватный ключ
- рассчитывает детерминированный хэш диплома
- собирает и шифрует QR payload
- подписывает JWT алгоритмом `Ed25519`
- пишет результат в `diploma_hashes` и `batch_results`
- обновляет прогресс батча
- публикует событие в `diplomas.processing_results`

### 4. Public Verification Service

`pubver` отвечает за:

- `GET /api/v1/verify?payload=<jwt>`
- `GET /api/v1/verify/search?diploma_number=<number>&vuz_code=<code>`
- публичный rate limit
- cache результатов проверки
- публикацию verification events в Kafka

`pubver` проверяет JWT подпись по публичному ключу вуза, расшифровывает `enc`, пересчитывает хэш и сверяет его с реестром в БД.

## Потоки данных

### Загрузка батча дипломов

1. Вуз вызывает `POST /api/v1/diplomas/upload`.
2. `gateway-service` валидирует записи.
3. Создаётся запись в `batches`.
4. Исходные данные каждой строки шифруются и сохраняются в `batch_record_payloads`.
5. В Kafka публикуются сообщения `diplomas.raw_tasks`.
6. `cryptography-engine-rs` обрабатывает каждую запись.
7. Результаты пишутся в `batch_results` и `diploma_hashes`.
8. `batches.processed_records` увеличивается до завершения батча.
9. Во frontend вуз видит прогресс батча и может скачать Excel.

### Публичная проверка по QR

1. Пользователь сканирует QR диплома.
2. Открывается маршрут `frontend /verify?payload=...`.
3. Frontend отправляет payload в `pubver`.
4. `pubver`:
   - без доверия читает `vuz_id` из JWT;
   - получает `universities.public_key`;
   - проверяет подпись `Ed25519`;
   - расшифровывает `enc`;
   - пересчитывает хэш;
   - проверяет статус в `diploma_hashes`.

### Публичная проверка по коду вуза и номеру диплома

1. Пользователь вводит `vuz_code` и `diploma_number`.
2. `pubver` ищет запись через `universities` и `diploma_hashes`.
3. Метаданные диплома восстанавливаются из последнего `batch_results.qr_payload`.

### Student portal

1. Студент ищет диплом через `GET /api/v1/student/search`.
2. `gateway-service` находит диплом по `diploma_number + full_name`.
3. Студент может:
   - выпустить share-link;
   - заново получить QR;
   - открыть публичную карточку share-link без входа в систему.

## PostgreSQL: основные таблицы

- `platform_admins` — администраторы платформы
- `universities` — карточка вуза, email, статус, публичный ключ
- `api_keys` — ERP API keys вузов
- `university_signing_keys` — зашифрованные приватные ключи вузов
- `batches` — батчи загрузки дипломов
- `batch_record_payloads` — зашифрованные исходные строки батча
- `batch_results` — результат обработки каждой строки батча
- `diploma_hashes` — реестр хэшей дипломов и их статусов
- `share_links` — публичные share-ссылки студентов
- `verification_events` — события публичных проверок для аналитики
- `file_schema_migrations` — служебная таблица кастомного мигратора

## Что хранится в открытом виде, а что шифруется

### Шифруется в БД

- `university_signing_keys.encrypted_private_key`
  Внутри: PEM приватного ключа вуза.

- `batch_record_payloads.encrypted_payload`
  Внутри: JSON с исходными полями диплома:
  `full_name`, `diploma_number`, `specialty`, `degree`, `faculty`, `year`.

### Хранится в подписанном и зашифрованном QR/JWT

Верхний JWT содержит:

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
  "full_name": "Иван Иванов Иванович",
  "diploma_number": "ДВС-2024-001234",
  "specialty": "Программная инженерия",
  "degree": "Бакалавр",
  "faculty": "ИиВТ",
  "year": 2026,
  "salt": "<hex>"
}
```

## Криптография

### Детерминированный хэш диплома

Каноническая строка:

```text
diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt
```

Хэш:

```text
SHA-256(canonical_string)
```

### Соль

Соль рассчитывается детерминированно:

```text
salt = SHA-256("diasoft|vuz_id|diploma_number")
```

Это даёт:

- стабильный хэш для повторной загрузки того же диплома;
- отсутствие случайных расхождений между сервисами;
- защиту от простого сопоставления одинаковых записей разных вузов.

### Подпись

- алгоритм: `Ed25519`
- JWT header: `alg = EdDSA`
- приватный ключ хранится только в зашифрованном виде
- публичный ключ хранится в `universities.public_key`

### Шифрование

Используется `AES-256-GCM`.

Сейчас один и тот же симметричный ключ `QR_PAYLOAD_ENCRYPTION_SECRET` используется для:

- шифрования `enc` внутри QR/JWT;
- шифрования `batch_record_payloads.encrypted_payload`.

Отдельный ключ `SIGNING_KEYS_MASTER_KEY` используется для:

- шифрования `university_signing_keys.encrypted_private_key`.

## Kafka

### Topic-ы

- `diplomas.raw_tasks` — задачи на криптообработку
- `diplomas.processing_results` — результаты обработки строк батча
- `verification.events` — события публичной проверки и аналитики

### Зачем Kafka

- разгружает `gateway-service` от тяжёлой криптообработки
- даёт асинхронный batch pipeline
- позволяет масштабировать воркер отдельно
- позволяет собирать аналитику независимо от frontend/backend запросов

## Миграции

В репозитории есть исторические дубли по номерным префиксам SQL-файлов миграций, поэтому для деплоя используется не `golang-migrate`, а кастомный file-based runner:

- `deploy/migrate.sh`
- служебная таблица `file_schema_migrations`

Это сделано специально, чтобы стек поднимался предсказуемо даже после merge старых веток.

`deploy/migrate-go.sh` оставлен только как совместимый wrapper и делегирует выполнение в `deploy/migrate.sh`.

## Подготовка секретов

Обязательные секреты:

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `SHARE_JWT_SECRET`
- `SIGNING_KEYS_MASTER_KEY`
- `QR_PAYLOAD_ENCRYPTION_SECRET`

Ключи `SIGNING_KEYS_MASTER_KEY` и `QR_PAYLOAD_ENCRYPTION_SECRET` должны быть:

- в `base64`
- после декодирования ровно `32 bytes`

### Генерация ключей

Linux/macOS:

```bash
openssl rand -base64 32
```

PowerShell:

```powershell
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes)
```

## Infisical: реальное подключение

В проект добавлена настоящая deploy-интеграция с Infisical через CLI и Machine Identity.

Что именно реализовано:

- [`deploy/infisical-compose.sh`](./deploy/infisical-compose.sh) — Linux/macOS wrapper
- [`deploy/infisical-compose.ps1`](./deploy/infisical-compose.ps1) — PowerShell wrapper
- [`\.env.infisical.example`](./.env.infisical.example) — шаблон настроек подключения к Infisical

Сценарий работы такой:

1. скрипт читает локальный `.env.infisical`;
2. при необходимости логинится в Infisical через Universal Auth;
3. делает `infisical export --format=dotenv`;
4. объединяет экспортированные секреты с локальным `.env.machine`;
5. запускает `docker compose` уже с итоговым env-файлом.

То есть секреты реально тянутся из Infisical при деплое, а не копируются руками в compose.

### Что хранить в Infisical

Минимальный набор секретов:

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `SHARE_JWT_SECRET`
- `SIGNING_KEYS_MASTER_KEY`
- `QR_PAYLOAD_ENCRYPTION_SECRET`
- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`

Опционально тоже можно хранить в Infisical:

- `PUBLIC_BASE_URL`
- `KAFKA_EXTERNAL_HOST`
- `TRUSTED_PROXY_CIDRS`
- `DEMO_UNIVERSITY_*`
- `GATEWAY_*`, `PUBVER_*`, `CRYPTO_*` runtime flags

### Env-файлы

В репозитории теперь есть три полезных шаблона:

- [`.env.example`](./.env.example) — полный пример для обычного env-based запуска без Infisical
- [`.env.machine.example`](./.env.machine.example) — локальный runtime env для сервера
- [`.env.infisical.example`](./.env.infisical.example) — настройки подключения к Infisical

Рекомендуемая схема для сервера:

1. `.env.machine` — несекретные локальные значения или override
2. `.env.infisical` — project/env/path и auth к Infisical
3. реальные секреты — внутри Infisical

### Настройка `.env.infisical`

Создать файл:

```bash
cp .env.infisical.example .env.infisical
```

Минимально заполнить:

```dotenv
INFISICAL_API_URL=https://app.infisical.com
INFISICAL_PROJECT_ID=...
INFISICAL_ENV_SLUG=prod
INFISICAL_SECRET_PATH=/
INFISICAL_UNIVERSAL_AUTH_CLIENT_ID=...
INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET=...
```

Если используешь service token вместо Universal Auth:

```dotenv
INFISICAL_PROJECT_ID=...
INFISICAL_ENV_SLUG=prod
INFISICAL_SECRET_PATH=/
INFISICAL_SERVICE_TOKEN=...
```

### Настройка `.env.machine`

Создать файл:

```bash
cp .env.machine.example .env.machine
```

Оставить там только то, что не обязательно хранить в Infisical. Например:

```dotenv
PUBLIC_BASE_URL=https://your-domain.example.com
KAFKA_EXTERNAL_HOST=your-domain.example.com
TRUSTED_PROXY_CIDRS=127.0.0.1/32,10.0.0.0/8
GATEWAY_LOG_LEVEL=info
PUBVER_LOG_LEVEL=info
CRYPTO_RUST_LOG=info,rdkafka=warn,sqlx=warn
```

Если один и тот же ключ есть и в `.env.machine`, и в Infisical, итоговое значение возьмётся из Infisical.

## Пошаговый деплой

### Вариант 1. Рекомендуемый: через Infisical

#### Linux / macOS

```bash
cp .env.infisical.example .env.infisical
cp .env.machine.example .env.machine

# заполнить оба файла

chmod +x deploy/infisical-compose.sh
./deploy/infisical-compose.sh run --rm db-migrate
./deploy/infisical-compose.sh up -d --build
```

#### Windows PowerShell

```powershell
Copy-Item .env.infisical.example .env.infisical
Copy-Item .env.machine.example .env.machine

# заполнить оба файла

.\deploy\infisical-compose.ps1 run --rm db-migrate
.\deploy\infisical-compose.ps1 up -d --build
```

#### Проверка

```bash
./deploy/infisical-compose.sh ps
./deploy/infisical-compose.sh logs --tail=100 gateway-service
./deploy/infisical-compose.sh logs --tail=100 cryptography-engine-rs
./deploy/infisical-compose.sh logs --tail=100 pubver
```

### Вариант 2. Без Infisical

Если Infisical не используется, можно по-прежнему поднять всё через обычный env-файл:

```bash
cp .env.example .env.machine
# заполнить секреты руками
docker compose --env-file .env.machine run --rm db-migrate
docker compose --env-file .env.machine up -d --build
```

## Внешние порты

- `http://<host>` — frontend
- `http://<host>:8080` — gateway-service
- `http://<host>:8081` — kafka-ui
- `http://<host>:8082` — pubver
- `postgres` — `5432`
- `kafka` — `9092`
- `redis` — `6379`

## Прод-заметки

- В production не оставляй demo-учётки и demo private key в env.
- Для первого запуска обязательно задай `BOOTSTRAP_ADMIN_EMAIL` и `BOOTSTRAP_ADMIN_PASSWORD`.
- Если frontend публикуется наружу, лучше ставить внешний reverse proxy с HTTPS.
- Предупреждение Chrome про `blob loaded over insecure connection` исчезнет только после HTTPS.

## Что проверить перед защитой

1. Вуз может загрузить signing key.
2. Загрузка дипломов без signing key запрещена.
3. Батч уходит в Kafka и доходит до `completed` или `failed`.
4. Excel-выгрузка соответствует конкретному `batch_id`.
5. QR-проверка проходит через `pubver`.
6. Проверка по `vuz_code + diploma_number` работает.
7. Student portal умеет выпускать share-link и повторный QR.
8. В логах сервисов нет запуска на пустых секретах.
