# agents.md

## Назначение документа

Этот файл фиксирует актуальный рабочий контекст проекта `pubver`, чтобы при следующих изменениях не пришлось заново восстанавливать архитектурные договоренности.

Документ описывает:

- роль сервиса;
- границы ответственности;
- подтвержденный контракт публичной проверки;
- используемые таблицы БД;
- важные инварианты, которые нельзя ломать.

## Краткое описание сервиса

`pubver` — публичный read-only сервис для проверки диплома по QR JWT и для поиска по `diploma_number + vuz_code`.

Сервис не выпускает дипломы и не изменяет реестр. Он только:

1. принимает JWT из QR;
2. извлекает `vuz_id`;
3. получает `public_key` ВУЗа из БД;
4. проверяет `RS256` подпись JWT;
5. пересчитывает `SHA-256` по payload;
6. ищет хеш в БД;
7. возвращает статус диплома.

## Источник истины

Для публичной проверки источником истины является не сам JWT, а связка:

- валидный `RS256` JWT;
- корректно пересчитанный хеш;
- наличие записи в `diploma_hashes`;
- статус записи в `diploma_hashes.status`.

JWT сам по себе не подтверждает действительность диплома без lookup в БД.

## Подтвержденный verify-поток

Поток `GET /api/v1/verify?payload=<jwt>` должен работать так:

1. Декодировать payload без доверия к нему.
2. Достать `vuz_id`.
3. Найти `universities.public_key`.
4. Проверить `RS256` подпись JWT.
5. Из валидного JWT извлечь:
   - `student_name` или `full_name`
   - `diploma_number`
   - `specialty`
   - `year`
   - `vuz_id`
   - `salt`
6. Собрать строку:

```text
full_name|diploma_number|specialty|year|vuz_id|salt
```

7. Посчитать `SHA-256`.
8. Сравнить полученный хеш с `sub` и `diploma_hash`, если они есть в JWT.
9. Выполнить lookup в `diploma_hashes.hash`.
10. Вернуть статус `active`, `revoked` или `not_found`.

## Подтвержденный search-поток

Поток `GET /api/v1/verify/search?diploma_number=&vuz_code=` должен:

1. принимать `diploma_number`;
2. принимать `vuz_code`;
3. искать запись через `universities.vuz_code + diploma_hashes.diploma_number`;
4. возвращать только публичные поля.

## Важные инварианты

- Публичная проверка должна использовать именно `RS256` для JWT.
- `universities.public_key` в текущей модели — это RSA public key для проверки QR JWT.
- Публичная проверка не должна использовать `diploma_hashes.signature`.
- Публичная проверка не должна расшифровывать `encrypted_payload`.
- Финальная валидность диплома определяется не JWT, а записью в БД.
- Алгоритм хеширования в `pubver` и Crypto Engine должен быть одинаковым.

## Алгоритм хеширования

Контракт хеширования подтвержден такой:

```text
raw = full_name|diploma_number|specialty|year|vuz_id|salt
hash = SHA-256(raw)
```

Результат хранится как hex-строка в нижнем регистре.

## Таблицы БД, которые реально нужны `pubver`

### `universities`

Используются поля:

- `id`
- `name`
- `public_key`
- `vuz_code`

### `diploma_hashes`

Используются поля:

- `hash`
- `vuz_id`
- `diploma_number`
- `status`
- `revoked_at`

### `diploma_publications`

Используются поля:

- `diploma_hash`
- `graduate_year`
- `specialty`

## Что не нужно придумывать без явного решения

Нельзя считать подтвержденным без отдельной договоренности:

- использование Ed25519 для публичной проверки;
- отдельную проверку `diploma_hashes.signature`;
- хранение нескольких типов ключей в `universities`;
- обязательность `exp` для QR JWT;
- дополнительные claims вроде `iss`, `aud`, `kid`, если они еще не закреплены контрактом.

## Текущий вывод для дальнейшей разработки

Если меняется схема публичной проверки, в первую очередь нужно синхронно обновлять:

- `README.md`
- `docs/architecture.md`
- `openapi/public-verification.yaml`
- `pkg/verifyhash/*`
- `internal/service/verification_service.go`
- `internal/repository/postgres/verification_repository.go`
