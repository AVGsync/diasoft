# Pubver - Code Review Problems

Статус:
- `[Исправлено]` - проблема закрыта в коде

## Ошибки

### 1. `University.Name` без `sql.NullString` - упадёт при `NULL` в БД `[Исправлено]`

**Файл:** `internal/repository/postgres/verification_repository.go`

```go
var universityCode sql.NullString
var universityName sql.NullString
```

Исправлено через чтение `u.name` в `sql.NullString` и безопасное присваивание в модель.

---

### 2. `ErrDiplomaRecordNotFound` объявлена, но нигде не используется `[Исправлено]`

**Файл:** `internal/domain/errors.go`

```go
ErrDiplomaRecordNotFound = errors.New("diploma record not found")
```

Исправлено: репозиторий теперь возвращает `ErrDiplomaRecordNotFound` вместо неявного `(nil, nil)`, а сервис обрабатывает это как `not_found`.

---

## Неточности

### 3. Ложноположительное декодирование base64 в `enc` `[Исправлено]`

**Файл:** `pkg/verifyhash/a256gcm.go`

Раньше декодер перебирал несколько base64-форматов подряд.  
Исправлено: теперь используется один фиксированный декодер под контракт `base64(nonce|ciphertext|tag)`.

---

### 4. `toVerifyResponse` брал данные из JWT, а не из БД `[Исправлено]`

**Файл:** `internal/service/verification_service.go`

Исправлено: `VerifyResponse` теперь собирается из данных реестра, а не из расшифрованного JWT payload.

---

### 5. При `record == nil` API отдавало персональные данные из JWT `[Исправлено]`

**Файл:** `internal/service/verification_service.go`

Исправлено: при `not_found` сервис возвращает только минимальный ответ:

```json
{
  "valid": false,
  "status": "not_found",
  "hash": "..."
}
```

---

### 6. `writeJSON` писал заголовки до кодирования тела `[Исправлено]`

**Файл:** `internal/httpapi/router.go`

Исправлено: JSON сначала кодируется в буфер, и только потом отправляется клиенту.

---

### 7. `envIntOrDefault` молча глотал ошибки парсинга `[Исправлено]`

**Файл:** `internal/config/config.go`

Исправлено: при невалидном значении переменной окружения функция теперь возвращает ошибку.

---

## Мелкие замечания

### 8. `extractInt64Claim` - потенциальная потеря точности `float64 -> int64` `[Исправлено]`

**Файл:** `pkg/verifyhash/qr_jwt.go`

Исправлено: добавлена проверка safe integer range перед преобразованием `float64` в `int64`.

---

### 9. Дублирование валидации между `EncryptedDiplomaPayload` и `DiplomaHashInput` `[Исправлено]`

**Файлы:** `pkg/verifyhash/a256gcm.go`, `pkg/verifyhash/hash.go`

Исправлено: общая валидация вынесена в единый helper и переиспользуется в обоих местах.

---

### 10. Несогласованность `omitempty` в JSON-тегах `[Исправлено]`

**Файл:** `internal/domain/model.go`

Исправлено: опциональные поля ответов приведены к единому стилю через `omitempty`.

---

### 11. Нет rate limiting `[Исправлено]`

**Файлы:** `internal/httpapi/middleware.go`, `internal/config/config.go`

Исправлено: добавлен per-IP rate limiting для публичных ручек с `429 Too Many Requests`,
`Retry-After` и настройкой через env:

- `RATE_LIMIT_ENABLED`
- `RATE_LIMIT_RPS`
- `RATE_LIMIT_BURST`
- `RATE_LIMIT_VISITOR_TTL`
- `RATE_LIMIT_CLEANUP_INTERVAL`
