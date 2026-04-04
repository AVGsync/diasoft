# Diasoft Platform

Корневой `docker compose` находится в [docker-compose.yml](/D:/Project_Go/diasoft/docker-compose.yml). Стек поднимает `postgres`, `kafka`, `gateway-service`, `cryptography-engine-rs`, `pubver` и `frontend`.

Быстрый старт:

```bash
docker compose up --build
```

Перед запуском должен быть включён Docker Desktop / Linux engine.

Основные адреса:

- `http://localhost:3000` — React-фронтенд
- `http://localhost:8080` — `gateway-service`
- `http://localhost:8082` — `pubver`

Тестовые учётки:

- Администратор: `admin@platform.local` / `Admin12345!`
- Вуз: `demo.vuz@platform.local` / `University123!`
- Код демо-вуза для публичного поиска: `DEMO2026`

Что изменено в стенде:

- публичная проверка вынесена из `gateway-service` в `pubver`
- демо-вуз создаётся автоматически при старте `gateway-service`
- demo signing key подгружается автоматически из [`deploy/demo/demo_university_ed25519_private_key.pem`](/D:/Project_Go/diasoft/deploy/demo/demo_university_ed25519_private_key.pem)
- фронтенд проксирует запросы на `gateway-service` и `pubver`, поэтому отдельная CORS-настройка не нужна
