# Redis Instruction

## Что у нас Redis делает в проекте

Сейчас Redis используется для двух вещей:

1. `rate limiter`
2. `cache`

Разделение по namespace такое:

- limiter: `rl:*`
- cache: `cache:*`

То есть один Redis общий, но ключи логически разделены.

## Зачем это нужно

### Rate limiter

Нужен, чтобы:

- не дать бесконтрольно спамить API;
- защитить публичные и чувствительные ручки;
- не положить PostgreSQL и сервисы лишними запросами.

### Cache

Нужен, чтобы:

- уменьшить число одинаковых чтений из PostgreSQL;
- ускорить ответы;
- лучше переживать всплески нагрузки.

## Что вам нужно сделать один раз

### 1. Поднять Redis

Из корня проекта:

```powershell
cd d:\diasoft
docker compose up -d redis
```

Проверить:

```powershell
docker compose ps redis
docker compose logs -f redis
```

### 2. Поднять сервисы

Если хотите только основные сервисы:

```powershell
cd d:\diasoft
docker compose up -d --build redis gateway-service pubver
```

Если хотите весь проект:

```powershell
cd d:\diasoft
docker compose up -d --build
```

### 3. Подключиться к Redis через GUI

Сейчас Redis проброшен наружу на `localhost:6379`.

Подключение:

- Host: `127.0.0.1`
- Port: `6379`
- Username: пусто
- Password: `diasoft-rate-limit-secret`
- Database: `0`

Если GUI просит URI:

```text
redis://:diasoft-rate-limit-secret@127.0.0.1:6379/0
```

## Как проверить rate limiter

### Через `pubver`

Сделайте несколько запросов:

```powershell
curl "http://localhost:8082/api/v1/verify?payload=test"
curl "http://localhost:8082/api/v1/verify?payload=test"
curl "http://localhost:8082/api/v1/verify/search?diploma_number=123&vuz_code=001X7276"
```

Потом посмотрите ключи:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:*"
```

Ожидаемые ключи:

- `rl:pubver:verify:ip:...`
- `rl:pubver:search:ip:...`

### Через `gateway-service`

Сделайте запросы к `auth`, `student`, `admin` или `diplomas`.

Потом:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:gateway:*"
```

## Как проверить cache

### `pubver`

Сделайте несколько одинаковых запросов:

```powershell
curl "http://localhost:8082/api/v1/verify/search?diploma_number=123&vuz_code=001X7276"
```

Потом:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:pubver:*"
```

### `gateway-service`

Сделайте несколько одинаковых запросов, например:

- `GET /api/v1/admin/stats`
- `GET /api/v1/admin/universities`
- `GET /api/v1/vuz/profile`
- `GET /api/v1/diplomas/batches/{batch_id}`

Потом:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:gateway:*"
```

## Полезные команды

Все limiter-ключи:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:*"
```

Все cache-ключи:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:*"
```

Посмотреть конкретный ключ:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret GET "cache:gateway:admin:stats"
```

Для limiter-ключей обычно используется `HASH`, поэтому смотреть так:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret HGETALL "rl:pubver:verify:ip:172.18.0.1"
```

Удалить ключ вручную:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret DEL "cache:gateway:admin:stats"
```

Очистить только cache:

```powershell
docker compose exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:*" | ForEach-Object { docker compose exec redis redis-cli -a diasoft-rate-limit-secret DEL $_ }
```

## На что смотреть в GUI

### Для rate limiter

Ищите:

- `rl:pubver:*`
- `rl:gateway:*`

Это временные ключи с TTL.

### Для cache

Ищите:

- `cache:pubver:*`
- `cache:gateway:*`

Это тоже временные ключи, но они могут жить дольше, чем limiter.

## Если ключей не видно

Проверьте по порядку:

1. Redis реально запущен:

```powershell
docker compose ps redis
```

2. Сервисы пересобраны после изменений:

```powershell
docker compose up -d --build redis gateway-service pubver
```

3. Запросы действительно отправлялись в сервисы.

4. Смотрите именно `DB 0`.

5. В GUI включён refresh.

## Что вам обычно не нужно трогать

Если проект уже поднят через [docker-compose.yml](/d:/diasoft/docker-compose.yml), обычно вам не нужно:

- вручную создавать базы в Redis GUI;
- вручную создавать ключи;
- вручную чистить limiter keys.

Redis в проекте работает сам:

- limiter keys создаются при запросах;
- cache keys создаются при чтении;
- старые ключи исчезают по TTL или инвалидации.
