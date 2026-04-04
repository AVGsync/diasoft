# Redis Cache

## Зачем вообще нужен cache

Redis-кэш нужен, чтобы не ходить в PostgreSQL на каждый одинаковый read-запрос.

Для нашего проекта это даёт три основных эффекта:

1. Снижает нагрузку на PostgreSQL.
2. Ускоряет ответы на часто повторяющиеся запросы.
3. Делает систему стабильнее под всплесками трафика.

При этом кэш у нас сделан аккуратно:

- кэшируются только безопасные read-heavy сценарии;
- чувствительные расшифрованные payload не складываются в Redis;
- для изменяемых данных стоят короткие TTL;
- на важных write-операциях есть инвалидация.

## Как работает кэш

Используется схема `cache-aside`:

1. Сервис сначала пытается прочитать данные из Redis.
2. Если ключа нет, идёт в PostgreSQL.
3. Успешный результат кладётся в Redis с TTL.
4. При мутациях связанные ключи удаляются, чтобы не раздавать устаревшие данные.

## Что кэшируется в `pubver`

Файлы:

- [cached_repository.go](/d:/diasoft/pubver/internal/repository/cached_repository.go)
- [redis.go](/d:/diasoft/pubver/internal/rediscache/redis.go)
- [config.go](/d:/diasoft/pubver/internal/config/config.go)
- [main.go](/d:/diasoft/pubver/cmd/pubver/main.go)

Кэшируются:

- `FindUniversityVerificationKeyByID`
- `FindByHash`
- `FindByDiplomaNumber`

Ключи:

- `cache:pubver:university_verification_key:<vuz_id>`
- `cache:pubver:diploma_hash:<hash>`
- `cache:pubver:diploma_search:<vuz_code>:<diploma_number>`

TTL:

- `CACHE_UNIVERSITY_KEY_TTL=30m`
- `CACHE_DIPLOMA_BY_HASH_TTL=1m`
- `CACHE_DIPLOMA_SEARCH_TTL=1m`

Почему TTL у дипломных записей короткий:

- статус диплома может смениться на `revoked`;
- write-поток происходит не в `pubver`, а в `gateway-service`;
- поэтому здесь безопаснее короткий TTL.

Что специально не кэшируется в `pubver`:

- расшифрованный `enc`
- raw JWT
- `invalid payload`
- `not_found`

## Что кэшируется в `gateway-service`

Файлы:

- [cache.go](/d:/diasoft/gateway-service/internal/service/cache.go)
- [redis.go](/d:/diasoft/gateway-service/internal/infrastructure/rediscache/redis.go)
- [apiserver.go](/d:/diasoft/gateway-service/internal/app/apiserver/apiserver.go)
- [config.go](/d:/diasoft/gateway-service/internal/app/apiserver/config.go)

Кэшируются:

- `AdminService.Stats`
- `AdminService.ListUniversities`
- `AdminService.GetUniversity`
- `UniversityCabinetService.Profile`
- `DiplomaService.GetBatch`

Ключи:

- `cache:gateway:admin:stats`
- `cache:gateway:admin:universities`
- `cache:gateway:admin:university:<university_id>`
- `cache:gateway:university:profile:<university_id>`
- `cache:gateway:diploma:batch:<vuz_id>:<batch_id>`

TTL:

- `CACHE_ADMIN_STATS_TTL=30s`
- `CACHE_UNIVERSITIES_LIST_TTL=1m`
- `CACHE_UNIVERSITY_PROFILE_TTL=5m`
- `CACHE_BATCH_STATUS_TTL=15s`

## Инвалидация

### В `gateway-service`

После:

- approve университета
- update status
- delete университета

удаляются:

- `admin:stats`
- `admin:universities`
- `admin:university:<id>`
- `university:profile:<id>`

После:

- upload batch
- revoke diploma
- `HandleProcessingResult`

удаляется:

- `admin:stats`

И дополнительно после `HandleProcessingResult` удаляется:

- `diploma:batch:<vuz_id>:<batch_id>`

### В `pubver`

Явной инвалидации сейчас нет, поэтому и выбраны короткие TTL для дипломных данных.

## Конфигурация `pubver`

Поддерживаются env:

- `CACHE_ENABLED`
- `CACHE_UNIVERSITY_KEY_TTL`
- `CACHE_DIPLOMA_BY_HASH_TTL`
- `CACHE_DIPLOMA_SEARCH_TTL`
- `CACHE_REDIS_ADDR`
- `CACHE_REDIS_PASSWORD`
- `CACHE_REDIS_DB`
- `CACHE_REDIS_PREFIX`
- `CACHE_REDIS_DIAL_TIMEOUT`
- `CACHE_REDIS_READ_TIMEOUT`
- `CACHE_REDIS_WRITE_TIMEOUT`

Пример:

```env
CACHE_ENABLED=true
CACHE_UNIVERSITY_KEY_TTL=30m
CACHE_DIPLOMA_BY_HASH_TTL=1m
CACHE_DIPLOMA_SEARCH_TTL=1m
CACHE_REDIS_ADDR=redis:6379
CACHE_REDIS_PASSWORD=diasoft-rate-limit-secret
CACHE_REDIS_DB=0
CACHE_REDIS_PREFIX=cache:pubver
```

## Конфигурация `gateway-service`

Поддерживаются env:

- `CACHE_ENABLED`
- `CACHE_ADMIN_STATS_TTL`
- `CACHE_UNIVERSITIES_LIST_TTL`
- `CACHE_UNIVERSITY_PROFILE_TTL`
- `CACHE_BATCH_STATUS_TTL`
- `CACHE_REDIS_ADDR`
- `CACHE_REDIS_PASSWORD`
- `CACHE_REDIS_DB`
- `CACHE_REDIS_PREFIX`
- `CACHE_REDIS_DIAL_TIMEOUT`
- `CACHE_REDIS_READ_TIMEOUT`
- `CACHE_REDIS_WRITE_TIMEOUT`

Пример:

```env
CACHE_ENABLED=true
CACHE_ADMIN_STATS_TTL=30s
CACHE_UNIVERSITIES_LIST_TTL=1m
CACHE_UNIVERSITY_PROFILE_TTL=5m
CACHE_BATCH_STATUS_TTL=15s
CACHE_REDIS_ADDR=redis:6379
CACHE_REDIS_PASSWORD=diasoft-rate-limit-secret
CACHE_REDIS_DB=0
CACHE_REDIS_PREFIX=cache:gateway
```

## Docker Compose

В [docker-compose.yml](/d:/diasoft/docker-compose.yml) кэш уже подключён:

- `gateway-service` использует `cache:gateway`
- `pubver` использует `cache:pubver`

Redis при этом общий, но namespace разделены:

- rate limiter: `rl:*`
- cache: `cache:*`

Это сделано, чтобы limiter и cache не мешали друг другу.

## Как посмотреть ключи

Все cache-ключи:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:*"
```

Только `pubver`:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:pubver:*"
```

Только `gateway-service`:

```powershell
docker compose -f d:\diasoft\docker-compose.yml exec redis redis-cli -a diasoft-rate-limit-secret --scan --pattern "cache:gateway:*"
```

## Что пока специально не делали

Пока не кэшируются:

- `ListBatches`
- публичный поиск студентов
- расшифрованные дипломные payload
- большие Excel/download-артефакты

Это осознанно: сначала безопасный первый слой кэша, потом уже расширение по результатам нагрузки и hit rate.
