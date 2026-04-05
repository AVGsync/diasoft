# GeoLite2 для локальной аналитики

В эту папку нужно положить MaxMind database файл:

```text
GeoLite2-City.mmdb
```

Локальный путь на вашей машине:

```text
d:\diasoft\geoip\GeoLite2-City.mmdb
```

В контейнер `pubver` этот файл монтируется как:

```text
/app/geoip/GeoLite2-City.mmdb
```

## Зачем это нужно

Файл `GeoLite2-City.mmdb` нужен для GeoIP-обогащения аналитических событий.

Если база подключена, `pubver` сможет при обработке запроса:

- определить страну;
- попытаться определить город;
- записать эти данные в Kafka-событие;
- после этого `gateway-service` сохранит их в таблицу `verification_events`;
- и блок `География запросов` начнёт заполняться в аналитике.

## Что будет, если файла нет

Если `GeoLite2-City.mmdb` отсутствует:

- `pubver` всё равно запустится;
- аналитика в целом продолжит работать;
- но поля `country` и `city` будут пустыми;
- в логах `pubver` появится warning о том, что GeoIP resolver отключён.

То есть отсутствие файла не ломает сервис, а только отключает географию.

## Что уже настроено в проекте

В проекте уже добавлено всё необходимое:

- в `docker-compose.yml` папка `./geoip` монтируется в `pubver`;
- в `pubver` задан путь:

```text
ANALYTICS_GEOIP_DB_PATH=/app/geoip/GeoLite2-City.mmdb
```

- для локальной проверки через `X-Forwarded-For` добавлен dev-режим:

```text
TRUSTED_PROXY_CIDRS=127.0.0.1/32,172.16.0.0/12
```

Это сделано именно для локального Docker-стенда, чтобы можно было тестировать географию с localhost.

## Как включить географию

1. Убедитесь, что файл лежит здесь:

```text
d:\diasoft\geoip\GeoLite2-City.mmdb
```

2. Пересоздайте `pubver`, чтобы контейнер подхватил базу:

```powershell
cd d:\diasoft
docker compose up -d --build --force-recreate pubver
```

3. Проверьте логи:

```powershell
docker compose logs --tail=30 pubver
```

Если всё хорошо, warning про `geo resolver disabled` больше не будет.

## Почему на localhost география часто остаётся пустой

Обычный локальный запрос приходит не с публичного IP, а с private адреса:

- `127.0.0.1`
- `172.x.x.x`
- `192.168.x.x`

Для таких IP MaxMind географию не возвращает.

Поэтому на localhost географию нужно проверять не “обычным” запросом, а передавая публичный IP через `X-Forwarded-For`.

## Как проверить географию на localhost

### Вариант 1. Быстрая проверка через `curl`

Отправьте любой запрос в `pubver` с публичным IP в `X-Forwarded-For`.

Например:

```powershell
curl -H "X-Forwarded-For: 8.8.8.8" "http://localhost:8082/api/v1/verify?payload=test"
```

Даже если `verify` вернёт `400 invalid payload`, аналитическое событие всё равно будет создано.

Подойдут и другие публичные IP, например:

- `8.8.8.8`
- `1.1.1.1`
- любой реальный внешний IP, который вы хотите проверить

### Вариант 2. Проверка через браузер/Postman

В браузере напрямую заголовок `X-Forwarded-For` не подставить, поэтому для localhost-проверки удобнее:

- `curl`
- Postman
- Insomnia

В Postman:

1. Откройте запрос к `pubver`
2. Добавьте header:

```text
X-Forwarded-For: 8.8.8.8
```

3. Отправьте запрос

## Как проверить, что география реально записалась

После тестового запроса:

1. Подождите 1-3 секунды, чтобы событие прошло через Kafka consumer
2. Выполните:

```powershell
docker exec diasoft-postgres psql -U gateway_user -d postgres -c "SELECT created_at, country, city, status FROM verification_events ORDER BY id DESC LIMIT 5;"
```

Если GeoLite подключён и запрос шёл с публичным IP, в новых строках появятся:

- `country`
- `city`

## Как проверить через UI

После появления записей с географией:

1. Откройте `http://localhost:3000`
2. Войдите в кабинет ВУЗа или администратора
3. Перейдите в раздел аналитики
4. Посмотрите блок `География запросов`

Если всё настроено правильно, таблица перестанет быть пустой.

## Полезный тестовый сценарий

1. Перезапустить `pubver`

```powershell
docker compose up -d --build --force-recreate pubver
```

2. Отправить тестовый запрос:

```powershell
curl -H "X-Forwarded-For: 8.8.8.8" "http://localhost:8082/api/v1/verify?payload=test"
```

3. Проверить БД:

```powershell
docker exec diasoft-postgres psql -U gateway_user -d postgres -c "SELECT created_at, country, city, status FROM verification_events ORDER BY id DESC LIMIT 5;"
```

4. Открыть UI и посмотреть аналитику

## Важное замечание для production

Настройка

```text
TRUSTED_PROXY_CIDRS=127.0.0.1/32,172.16.0.0/12
```

подходит только для локального dev-окружения.

В production нужно указывать только реальные trusted reverse proxies или ingress CIDR, иначе клиент сможет подменять `X-Forwarded-For`.
