# Diasoft — Платформа верификации дипломов

Платформа для криптографически защищённой выдачи и публичной верификации дипломов об образовании. Вузы загружают реестры выпускников через API или веб-интерфейс, система подписывает каждый диплом ключом вуза (Ed25519) и генерирует QR-коды, по которым работодатели и студенты могут мгновенно проверить подлинность документа.

## Реализованная функциональность

- Загрузка дипломов в формате JSON и CSV батчами;
- Асинхронная криптографическая обработка через Kafka — подпись Ed25519, хэш SHA-256, шифрование AES-256-GCM;
- Публичная верификация дипломов по QR-коду (JWT payload) и по номеру диплома + коду вуза;
- Управление вузами: регистрация, подтверждение администратором, выпуск API-ключей для ERP-интеграции;
- Кабинет вуза: загрузка signing key, просмотр батчей, выгрузка Excel с QR-кодами;
- Поиск диплома студентом, генерация share-ссылки и QR для передачи работодателю;
- Отзыв диплома с изменением статуса в публичном реестре.

## Стек технологий

- **Backend:** Go (gateway-service, pubver), Rust (cryptography-engine)
- **Frontend:** React, TypeScript
- **База данных:** PostgreSQL
- **Очередь сообщений:** Apache Kafka
- **Кэш / rate-limit:** Redis
- **Инфраструктура:** Docker, Docker Compose
- **Криптография:** Ed25519, AES-256-GCM, SHA-256

## Сервисы

| Сервис | Описание |
|---|---|
| `gateway-service` | Основной HTTP API: авторизация, загрузка дипломов, кабинет вуза, поиск студента |
| `cryptography-engine-rs` | Rust-воркер: читает задачи из Kafka, подписывает QR, возвращает результаты |
| `pubver` | Публичная верификация дипломов по QR JWT и по номеру диплома |
| `frontend` | React SPA, раздаётся через nginx |
| `postgres` | Хранит вузы, батчи, хэши дипломов, ключи подписи |
| `kafka` | Транспорт между gateway и cryptography-engine |
| `redis` | Кэш и rate-limiting для pubver |

## Демо

Демо сервиса доступно по адресу: [http://89.108.113.181/](http://89.108.113.181/)

### Тестовые учётные записи

**Администратор платформы**
- Email: `admin@platform.local`
- Пароль: `Admin12345!`
- Описание: подтверждает заявки вузов, просматривает общую статистику реестра.

**Демо-вуз**
- Email: `demo.vuz@platform.local`
- Пароль: `University123!`
- Код вуза для публичной проверки: `DEMO2026`
- Описание: аккаунт активирован и готов к загрузке дипломов.

Тестовый CSV с дипломами: [`demo_diplomas_100.csv`](./deploy/demo/demo_diplomas_100.csv)

## Быстрый старт

### Требования

- Debian 9+ или любой debian-like Linux
- Docker 20+
- Docker Compose 2.0+
- Git

### Установка

```bash
sudo apt-get update && sudo apt-get install -y docker.io docker-compose git
git clone https://github.com/AVGsync/diasoft
cd diasoft
```

### Запуск

```bash
docker compose up --build -d
```

После запуска автоматически:
- применятся миграции базы данных (`db-migrate`);
- создадутся Kafka-топики (`kafka-init`);
- создастся администратор `admin@platform.local`;
- создастся демо-вуз `DEMO2026` с настроенным signing key.

### Основные адреса

| Адрес | Сервис |
|---|---|
| `http://localhost` | Фронтенд |
| `http://localhost:8080` | gateway-service API |
| `http://localhost:8082` | pubver (верификация) |
| `http://localhost:8081` | Kafka UI |

## Порядок подключения вуза

1. Вуз подаёт заявку через `POST /api/v1/auth/register`.
2. Администратор активирует вуз в кабинете.
3. Вуз логинится через `POST /api/v1/auth/login` и получает JWT.
4. Вуз загружает Ed25519 private key через `PUT /api/v1/vuz/signing-key`.
5. Вуз выпускает API-ключ для ERP через `POST /api/v1/vuz/api-keys`.
6. ERP загружает дипломы батчами и скачивает Excel с QR-кодами.

## API

### Авторизация

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/api/v1/auth/register` | Заявка вуза на подключение |
| POST | `/api/v1/auth/login` | Вход администратора или вуза |

### Кабинет вуза

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/v1/vuz/profile` | Профиль вуза |
| PUT | `/api/v1/vuz/signing-key` | Загрузка Ed25519 private key |
| GET | `/api/v1/vuz/signing-key` | Статус signing key |
| POST | `/api/v1/vuz/api-keys` | Выпуск API-ключа для ERP |
| GET | `/api/v1/vuz/api-keys` | Список API-ключей |
| GET | `/api/v1/vuz/batches` | История батчей |

### Дипломы

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/api/v1/diplomas/upload` | Загрузка батча (JSON или CSV) |
| GET | `/api/v1/diplomas/batches/{batch_id}` | Статус батча |
| GET | `/api/v1/diplomas/batches/{batch_id}/download` | Выгрузка Excel с QR |
| PATCH | `/api/v1/diplomas/{diploma_hash}/revoke` | Отзыв диплома |

### Студент и работодатель

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/v1/student/search` | Поиск диплома по номеру и/или ФИО |
| POST | `/api/v1/student/share` | Генерация share-ссылки |
| GET | `/api/v1/student/qr` | Повторная генерация QR |
| GET | `/api/v1/student/share/{token}` | Просмотр диплома по share-токену |

### Публичная верификация (pubver)

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/v1/verify?payload=<jwt>` | Проверка по QR JWT |
| GET | `/api/v1/verify/search` | Поиск по коду вуза и номеру диплома |

## Формат загрузки дипломов

**JSON:**
```json
{
  "diplomas": [
    {
      "full_name": "Иванов Иван Иванович",
      "diploma_number": "DVS-2026-0001",
      "specialty": "Программная инженерия",
      "degree": "Бакалавр",
      "faculty": "Факультет информационных технологий",
      "year": 2026
    }
  ]
}
```

**CSV:**
```
full_name,diploma_number,specialty,degree,faculty,year
```

Допустимые значения `degree`: `Бакалавр`, `Магистр`, `Специалист`.

