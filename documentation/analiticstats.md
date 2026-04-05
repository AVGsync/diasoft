# Аналитика проверок дипломов

## Что это за фича

В проект добавлена отдельная аналитическая цепочка для событий проверки дипломов.

Она собирает и агрегирует:

- сколько раз проверяли дипломы за период;
- распределение результатов проверки: `active`, `revoked`, `not_found`, `invalid_payload`, `invalid_input`, `internal_error`;
- динамику проверок по дням;
- географию запросов по стране и городу;
- топ университетов по числу проверок на уровне платформы.

Фича реализована не внутри одного `pubver`, а на уровне всей системы:

- `pubver` производит события проверок;
- `Kafka` принимает и буферизует поток событий;
- `gateway-service` потребляет эти события и сохраняет их в PostgreSQL;
- защищённые API ручки статистики отдаются из `gateway-service`;
- `frontend` может строить на их основе дашборды для админов и кабинета ВУЗа.

## Зачем это нужно

Эта функциональность нужна не только для “красивого графика”.

Она решает несколько практических задач:

- даёт ВУЗам понятную картину, насколько востребованы проверки их дипломов;
- показывает пики интереса к конкретным выпускам или массовые волны проверок;
- помогает замечать аномалии: всплеск `invalid_payload`, `not_found`, подозрительную активность из новых регионов;
- даёт платформе общий обзор использования сервиса и нагрузочного профиля;
- делает продукт сильнее с точки зрения B2B-ценности: это уже не просто реестр, а аналитическая система доверия.

## Почему сделано через Kafka

Аналитика сделана через Kafka, а не прямой записью в БД из `pubver`, потому что это правильнее архитектурно.

Плюсы такого решения:

- `pubver` остаётся быстрым и не блокируется тяжёлыми аналитическими запросами;
- публичный сервис верификации не становится жёстко связанным с дашбордом;
- события можно переиспользовать для других consumers: антифрод, алерты, BI, выгрузки;
- Kafka даёт буфер и развязку по времени между продюсером и консьюмером;
- систему проще масштабировать: можно увеличивать консьюмеры аналитики отдельно от публичного API.

Итоговая схема:

1. Пользователь делает запрос в `pubver`.
2. `pubver` возвращает результат проверки.
3. Параллельно `pubver` публикует событие в Kafka topic `verification.events`.
4. `gateway-service` читает событие из Kafka.
5. `gateway-service` сохраняет событие в таблицу `verification_events`.
6. API статистики читают агрегаты из PostgreSQL.

## Как это реализовано сейчас

### 1. Продюсер событий в `pubver`

`pubver` публикует в Kafka событие после:

- `GET /api/v1/verify`
- `GET /api/v1/verify/search`

В событие отправляются:

- `event_id`
- `created_at`
- `source_service`
- `endpoint`
- `request_id`
- `vuz_id`
- `vuz_code`
- `diploma_hash`
- `status`
- `is_valid`
- `country`
- `city`
- `client_ip_hash`
- `user_agent`

Что важно:

- исходный IP в Kafka не уходит, в событии хранится только его SHA-256 hash;
- расшифрованный `enc` и персональные поля диплома не отправляются в аналитику;
- география опциональна и заполняется только если у `pubver` подключён GeoIP database.

## Как включить географию запросов

Для заполнения блока `География запросов` нужен MaxMind database файл:

```text
GeoLite2-City.mmdb
```

Локально он ожидается здесь:

```text
d:\diasoft\geoip\GeoLite2-City.mmdb
```

В `docker-compose` он монтируется в контейнер `pubver` как:

```text
/app/geoip/GeoLite2-City.mmdb
```

Если файл отсутствует:

- `pubver` продолжит работать;
- аналитика в целом останется рабочей;
- но поля `country` и `city` будут пустыми.

## Почему на localhost география обычно пустая

При локальной разработке запросы часто приходят с private IP:

- `127.0.0.1`
- `172.x.x.x`
- `192.168.x.x`

Для таких адресов GeoLite не возвращает страну и город.

Поэтому на localhost географию надо проверять не “натуральным” адресом машины, а через прокинутый публичный IP в `X-Forwarded-For`.

## Как проверить географию на localhost

Для локального compose уже добавлено:

- `ANALYTICS_GEOIP_DB_PATH=/app/geoip/GeoLite2-City.mmdb`
- `TRUSTED_PROXY_CIDRS=127.0.0.1/32,172.16.0.0/12`

Это сделано только для dev-стенда, чтобы `pubver` принимал `X-Forwarded-For` от Docker bridge.

Порядок проверки:

1. Положить файл `GeoLite2-City.mmdb` в `d:\diasoft\geoip\`.
2. Пересоздать `pubver`:

```powershell
docker compose up -d --build --force-recreate pubver
```

3. Отправить тестовый запрос с публичным IP:

```powershell
curl -H "X-Forwarded-For: 8.8.8.8" "http://localhost:8082/api/v1/verify?payload=test"
```

Даже если сам verify вернёт `400 invalid payload`, аналитическое событие всё равно будет создано.

4. Подождать 1-3 секунды, пока событие пройдёт через Kafka consumer.
5. Открыть аналитику в кабинете ВУЗа или админа.

Также можно проверить напрямую в PostgreSQL:

```powershell
docker exec diasoft-postgres psql -U gateway_user -d postgres -c "SELECT created_at, country, city, status FROM verification_events ORDER BY id DESC LIMIT 5;"
```

Если всё подключено правильно, в новых событиях появятся `country` и `city`.

## Важное замечание по production

Широкий `TRUSTED_PROXY_CIDRS=172.16.0.0/12` добавлен только для локального Docker-стенда.

В production нужно указывать только реальные CIDR ваших trusted reverse proxies или ingress.

### 2. Kafka topic

Используется topic:

```text
verification.events
```

Он создаётся через `kafka-init` в `docker-compose`.

### 3. Консьюмер в `gateway-service`

В `gateway-service` поднят отдельный Kafka consumer для событий аналитики.

Он:

- читает `verification.events`;
- десериализует событие;
- дедуплицирует вставку по `event_id`;
- сохраняет данные в PostgreSQL.

### 4. Таблица PostgreSQL

Добавлена таблица:

```text
verification_events
```

Основные поля:

- `event_id`
- `created_at`
- `source_service`
- `endpoint`
- `request_id`
- `vuz_id`
- `vuz_code`
- `diploma_hash`
- `status`
- `is_valid`
- `country`
- `city`
- `client_ip_hash`
- `user_agent`

Индексы добавлены по:

- `created_at`
- `vuz_id + created_at`
- `vuz_code + created_at`
- `status + created_at`
- `endpoint + created_at`
- `country + created_at`

Это позволяет строить дашборд без полного скана таблицы.

## API статистики

Сейчас добавлены две защищённые ручки:

### Для кабинета ВУЗа

```http
GET /api/v1/vuz/stats/verifications
```

Доступ:

- только авторизованный ВУЗ;
- статистика возвращается только по его `vuz_id`.

### Для администратора платформы

```http
GET /api/v1/admin/stats/verifications
```

Доступ:

- только администратор платформы.

### Query параметры

Обе ручки поддерживают:

- `from`
- `to`

Форматы:

- `RFC3339`
- `YYYY-MM-DD`

Если диапазон не передан, сервис использует дефолтное окно в `gateway-service`.

## Что возвращает API статистики

Сейчас response включает:

- `from`
- `to`
- `total_checks`
- `unique_requesters`
- `statuses`
- `timeseries`
- `geography`
- `top_universities` — только для админского скоупа

То есть из этого уже можно построить:

- summary cards;
- график по дням;
- donut/bar chart по статусам;
- таблицу географии;
- рейтинг ВУЗов по числу проверок.

## Что уже проверено

На текущем этапе проверено следующее:

- `pubver` собирается и публикует аналитические события;
- `gateway-service` собирается и поднимает analytics consumer;
- миграция `verification_events` применяется;
- реальный запрос в `pubver` создаёт запись в `verification_events`.
- `GET /api/v1/admin/stats/verifications` возвращает агрегированный ответ по сохранённым событиям.

Проверочный сценарий уже отработал:

- запрос `GET /api/v1/verify?payload=test` вернул `400 invalid payload`;
- событие было опубликовано и сохранено в БД как `status = invalid_payload`.

## Что ещё можно сделать следующим этапом

Это уже не обязательно для MVP, но логично как следующий шаг:

- кэшировать тяжёлые analytics responses в Redis;
- добавить GeoIP database и заполнение `country/city`;
- добавить top diplomas и top programs;
- добавить anomaly detection по всплескам проверок;
- добавить экспорт отчётов;
- построить UI-дашборд в `frontend`.

## Что нужно для работы

### В `pubver`

Нужны env:

- `ANALYTICS_ENABLED=true`
- `ANALYTICS_KAFKA_BROKERS`
- `ANALYTICS_KAFKA_TOPIC=verification.events`
- `ANALYTICS_KAFKA_CLIENT_ID`
- `ANALYTICS_KAFKA_WRITE_TIMEOUT`
- `ANALYTICS_QUEUE_SIZE`

Опционально:

- `ANALYTICS_GEOIP_DB_PATH`

### В `gateway-service`

Нужны Kafka настройки:

- `verification_events_topic = "verification.events"`
- `verification_events_group = "gateway-service-verifications"`

### В инфраструктуре

Должны быть доступны:

- PostgreSQL
- Kafka
- `kafka-init`, создающий topic `verification.events`

## Короткий итог

Теперь аналитика проверок построена правильно с точки зрения архитектуры:

- не в лоб из `pubver` в БД;
- а через событийную схему `pubver -> Kafka -> gateway-service -> PostgreSQL -> stats API`.

Это решение даёт:

- меньшую связанность сервисов;
- лучшую масштабируемость;
- возможность расширять аналитику без переписывания публичного сервиса проверки.
