# Redis: кеш API, инвалидация, rate-limit

## 1. Роль Redis в платформе

Redis используется в двух ключевых задачах:

- защита логина (distributed rate limiting);
- кеширование GET API с версионной инвалидацией.

## 2. Клиент и настройки

Файл: `/backend/internal/cache/redis.go`

Параметры клиента:

- `DialTimeout: 5s`
- `ReadTimeout: 2s`
- `WriteTimeout: 2s`

## 3. Login rate limiter

Файл: `/backend/internal/security/redis_rate_limiter.go`

Механика:

- ключ: `rl:login:<ip:email>`;
- инкремент через Lua script `INCR + PEXPIRE`;
- окно и лимит настраиваются через ENV;
- fail-open поведение при Redis ошибке (чтобы не положить auth полностью).

ENV:

- `SEC_LOGIN_RATE_LIMIT`
- `SEC_LOGIN_RATE_WINDOW`

## 4. API Cache middleware

Файл: `/backend/internal/middleware/api_cache.go`

### 4.1 Что кешируется

- только `GET` запросы;
- без `/healthz`, `/metrics`, `/dev/swagger`;
- статус 2xx;
- body до `API_CACHE_MAX_BODY_BYTES`.

### 4.2 Cache key

Хэшируется комбинация:

- method
- path
- query
- tenant header
- global version
- tenant version
- user version

Ключ ответа: `api_cache:resp:<sha256>`.

### 4.3 Версии инвалидации

При успешном mutating запросе (`POST/PUT/PATCH/DELETE`):

- инкремент `api_cache:v:global`;
- инкремент `api_cache:v:tenant:<tenant_id>` (если есть);
- инкремент `api_cache:v:user:<user_id>` (если есть).

Это дает дешёвую логическую инвалидацию без scan/delete по префиксам.

### 4.4 Диагностический заголовок

Ответы GET получают header:

- `X-API-Cache: HIT` или `MISS`.

## 5. Где Redis не используется

Redis сейчас не является primary store для:

- сессий refresh token (они в PostgreSQL);
- бизнес-сущностей (alerts/cases/tasks/...);
- очередей ingestion.

## 6. Риски и эксплуатация

- При недоступности Redis:
  - rate-limit работает fail-open;
  - cache деградирует до passthrough.
- Для прод стоит добавить:
  - Redis monitoring/alerts;
  - eviction policy контроль;
  - отдельный namespace/DB для IncidentHub.
