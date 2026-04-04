# Gateway Service

> Update: `batch_records` is deprecated and removed by migration `009_remove_batch_records`.
> The service now keeps per-record processing state in `batch_results` and reads student data from `qr_payload`, so plaintext student data is no longer stored in PostgreSQL.

`gateway-service` это точка входа для ВУЗов и платформенный gateway для пакетной загрузки дипломов.

Сервис реализует:
- регистрацию и логин ВУЗов и администратора платформы с JWT;
- генерацию статических API-ключей для ERP-интеграций;
- прием JSON/CSV реестров выпускников;
- публикацию задач в Kafka (`diplomas.raw_tasks`);
- прием результатов крипто-обработки из Kafka (`diplomas.processing_results`);
- прогресс батчей, отзыв диплома, Excel-выгрузку с QR-кодами;
- поиск студента, временные share-link JWT и QR для студента;
- публичную проверку QR payload.

## Архитектура

За основу взят каркас из `d:\Project_Go\titanit\psyhologicApi`:
- `cmd/apiserver` только читает конфиг и стартует приложение;
- `internal/app/apiserver` собирает зависимости вручную;
- `internal/transport/http` содержит handlers и middleware;
- `internal/service` содержит бизнес-логику;
- `internal/repository/postgres` изолирует SQL и транзакции;
- `internal/infrastructure` содержит JWT, bcrypt, Kafka, Excel и QR.

Относительно исходной схемы данных добавлены две сущности:
- `platform_admins` для реального логина глобального администратора;
- `batch_records` для хранения исходных строк батча.

Без `batch_records` сервис не смог бы корректно:
- искать студента по ФИО и номеру диплома;
- формировать Excel после асинхронной обработки в Kafka;
- детерминированно сопоставлять `record_index` из результата с исходной записью.

## Локальный запуск

1. Поднять инфраструктуру:

```bash
docker compose up -d
```

2. Применить SQL-файлы из [`migrations`](./migrations) по порядку.

3. Запустить сервис:

```bash
go run ./cmd/apiserver -config-path configs/apiserver.toml
```

## Конфиг

Основные параметры лежат в [`configs/apiserver.toml`](./configs/apiserver.toml).

Поддерживаются env overrides:
- `DATABASE_URL`
- `JWT_SECRET`
- `SHARE_JWT_SECRET`
- `PUBLIC_BASE_URL`
- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`
- `KAFKA_BROKERS`
- `KAFKA_RAW_TOPIC`
- `KAFKA_RESULTS_TOPIC`
- `KAFKA_CONSUMER_GROUP`

## Основные ручки

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/admin/universities/{id}/approve`
- `GET /api/v1/admin/stats`
- `POST /api/v1/vuz/api-keys`
- `GET /api/v1/vuz/api-keys`
- `POST /api/v1/diplomas/upload`
- `GET /api/v1/diplomas/batches/{batch_id}`
- `GET /api/v1/diplomas/batches/{batch_id}/download`
- `PATCH /api/v1/diplomas/{diploma_hash}/revoke`
- `GET /api/v1/student/search`
- `POST /api/v1/student/share`
- `GET /api/v1/student/qr`
- `GET /api/v1/student/share/{token}`
- `GET /api/v1/verify`
