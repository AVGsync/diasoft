# Redis Rate Limiter

## Зачем вообще нужен rate limiter

Rate limiter нужен, чтобы публичные и чувствительные ручки нельзя было бесконтрольно спамить.

Для нашего проекта он нужен по нескольким причинам:

1. Защита от brute-force и перебора.
2. Защита от всплесков нагрузки и банального спама.
3. Снижение риска деградации PostgreSQL и сервисов.
4. Более предсказуемое поведение под нагрузкой.

Особенно это важно для:

- `pubver`, потому что это публичная поверхность;
- `gateway-service`, потому что там есть `auth`, `admin`, `upload`, `student` и batch API.

## Почему rate limiter сделан именно в Redis

Если лимитер хранить только в памяти процесса, он будет работать только внутри одного инстанса.

Redis даёт нам:

- общую точку хранения лимитов для нескольких инстансов;
- одинаковое поведение при масштабировании;
- TTL для автоматической очистки ключей;
- быстрые атомарные операции.

## Как работает текущий limiter

Используется `token bucket` на Lua-скрипте в Redis.

Логика такая:

1. Для каждого клиента и политики строится Redis-ключ.
2. В ключе хранятся:
   - `tokens`
   - `ts`
3. На каждый запрос лимитер:
   - вычисляет, сколько токенов успело восстановиться;
   - уменьшает bucket, если токены есть;
   - иначе возвращает `retry_after`.
4. Ключу выставляется TTL, чтобы Redis сам его удалял.

Это даёт:

- поддержку `burst`
- постепенное восстановление лимита
- отсутствие бесконечного роста ключей

## Где это реализовано

### `pubver`

Файлы:

- [middleware.go](/d:/diasoft/pubver/internal/httpapi/middleware.go)
- [router.go](/d:/diasoft/pubver/internal/httpapi/router.go)
- [config.go](/d:/diasoft/pubver/internal/config/config.go)
- [main.go](/d:/diasoft/pubver/cmd/pubver/main.go)

Лимитируются:

- `/api/v1/verify`
- `/api/v1/verify/search`

Не лимитируется:

- `/healthz`

Текущие политики:

- `verify`: `RATE_LIMIT_VERIFY_RPS=0.25`, `RATE_LIMIT_VERIFY_BURST=5`
- `search`: `RATE_LIMIT_SEARCH_RPS=0.1`, `RATE_LIMIT_SEARCH_BURST=3`

То есть:

- `verify` примерно 1 запрос в 4 секунды со всплеском до 5
- `search` примерно 1 запрос в 10 секунд со всплеском до 3

### `gateway-service`

Файлы:

- [ratelimit.go](/d:/diasoft/gateway-service/internal/transport/http/middleware/ratelimit.go)
- [apiserver.go](/d:/diasoft/gateway-service/internal/app/apiserver/apiserver.go)
- [config.go](/d:/diasoft/gateway-service/internal/app/apiserver/config.go)
- [main.go](/d:/diasoft/gateway-service/cmd/apiserver/main.go)

Лимитируются разные группы ручек с разной идентичностью:

- `auth` по IP
- `student` по IP
- `admin` по `admin_id`
- `vuz` по `university_id`
- `diplomas` по `api_key_id` или `university_id`

Это важный момент: не всё режется только по IP.

## Как строятся ключи

В Redis используются ключи с namespace `rl:*`.

Примеры:

- `rl:pubver:verify:ip:1.2.3.4`
- `rl:pubver:search:ip:1.2.3.4`
- `rl:gateway:auth_login:ip:1.2.3.4`
- `rl:gateway:admin:admin:<id>`
- `rl:gateway:university:university:<id>`
- `rl:gateway:diploma_upload:api_key:<id>`

Это позволяет отдельно контролировать разные типы трафика.

## Trusted proxy и защита от spoofing

Одна из типичных уязвимостей у rate limiter — слепое доверие `X-Forwarded-For`.

У нас это закрыто так:

- если запрос пришёл не от trusted proxy, берётся `RemoteAddr`;
- `X-Forwarded-For` и `X-Real-IP` учитываются только если peer IP входит в `TRUSTED_PROXY_CIDRS`.

То есть клиент снаружи не должен иметь возможность подделкой заголовка обойти лимит.

## Что возвращается при превышении лимита

При превышении:

- статус `429 Too Many Requests`
- заголовок `Retry-After`

Если сам Redis недоступен:

- сервис работает в fail-closed режиме;
- возвращается `503 rate limiter unavailable`

Это сделано специально: лучше отказать, чем внезапно снять защиту.

## Конфигурация `pubver`

Поддерживаются env:

- `RATE_LIMIT_ENABLED`
- `RATE_LIMIT_VERIFY_RPS`
- `RATE_LIMIT_VERIFY_BURST`
- `RATE_LIMIT_SEARCH_RPS`
- `RATE_LIMIT_SEARCH_BURST`
- `RATE_LIMIT_KEY_TTL`
- `RATE_LIMIT_REDIS_ADDR`
- `RATE_LIMIT_REDIS_PASSWORD`
- `RATE_LIMIT_REDIS_DB`
- `RATE_LIMIT_REDIS_PREFIX`
- `RATE_LIMIT_REDIS_DIAL_TIMEOUT`
- `RATE_LIMIT_REDIS_READ_TIMEOUT`
- `RATE_LIMIT_REDIS_WRITE_TIMEOUT`
- `TRUSTED_PROXY_CIDRS`

Пример:

```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_VERIFY_RPS=0.25
RATE_LIMIT_VERIFY_BURST=5
RATE_LIMIT_SEARCH_RPS=0.1
RATE_LIMIT_SEARCH_BURST=3
RATE_LIMIT_KEY_TTL=15m
RATE_LIMIT_REDIS_ADDR=redis:6379
RATE_LIMIT_REDIS_PASSWORD=diasoft-rate-limit-secret
RATE_LIMIT_REDIS_DB=0
RATE_LIMIT_REDIS_PREFIX=rl:pubver
```

## Конфигурация `gateway-service`

Поддерживаются env:

- `RATE_LIMIT_ENABLED`
- `RATE_LIMIT_KEY_TTL`
- `RATE_LIMIT_REDIS_ADDR`
- `RATE_LIMIT_REDIS_PASSWORD`
- `RATE_LIMIT_REDIS_DB`
- `RATE_LIMIT_REDIS_PREFIX`
- `RATE_LIMIT_REDIS_DIAL_TIMEOUT`
- `RATE_LIMIT_REDIS_READ_TIMEOUT`
- `RATE_LIMIT_REDIS_WRITE_TIMEOUT`
- `TRUSTED_PROXY_CIDRS`

Пример:

```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_KEY_TTL=15m
RATE_LIMIT_REDIS_ADDR=redis:6379
RATE_LIMIT_REDIS_PASSWORD=diasoft-rate-limit-secret
RATE_LIMIT_REDIS_DB=0
RATE_LIMIT_REDIS_PREFIX=rl:gateway
```

Сами политики `gateway-service` сейчас заданы в коде в [apiserver.go](/d:/diasoft/gateway-service/internal/app/apiserver/apiserver.go).

## Как посмотреть ключи limiter

Все limiter-ключи:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:*"
```

Только `pubver`:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:pubver:*"
```

Только `gateway-service`:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "rl:gateway:*"
```

## Что важно понимать

Rate limiter не заменяет:

- авторизацию
- валидацию
- аудит
- WAF / reverse proxy protection

Это только один слой защиты, но очень важный.

Для нас правильная комбинация такая:

- Redis-backed limiter в сервисах
- trusted proxy configuration
- короткий TTL на limiter keys
- разные политики для разных типов трафика
