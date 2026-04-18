# Безопасность и мультитенантность

## 1. Аутентификация

Используется JWT + refresh-token схема:

- Access token (короткий TTL) передается в `Authorization` header.
- Refresh token хранится в http-only cookie.
- Refresh tokens хешируются (`auth.RefreshHash`) и хранятся в БД.
- На refresh выполняется ротация с revoke старого токена.

Сессии пользователя:

- `GET /api/v1/auth/sessions`
- `POST /api/v1/auth/sessions/revoke-others`

Авто-истечение сессий определяется `AUTH_REFRESH_TTL`.

## 2. Пароли и LDAP

- локальные пароли: `argon2id` (`internal/security/password.go`)
- для LDAP-enabled пользователя проверка идет через LDAP authenticator
- LDAP конфигурация поднимается через ENV и имеет заготовку для интеграции

## 3. RBAC

Middleware:

- `RequirePlatformAdmin`
- `RequireTenantAdminOrPlatformAdmin`
- `RequireAnalystOrHigher`

Защита на backend является обязательной, frontend role-gating лишь UX-слой.

## 4. Tenant isolation

- tenant context обязателен для tenant routes (`X-Tenant-ID`)
- `ResolveTenant` валидирует принадлежность пользователя тенанту
- все репозитории фильтруют выборки по `tenant_id`
- IDs без tenant фильтра не дают доступ к чужим данным

## 5. Защита от brute-force

Login rate limit:

- Redis-based sliding window-like counter (`IncrWithWindow`)
- ключ `ip:email`
- регулируется `SEC_LOGIN_RATE_LIMIT`, `SEC_LOGIN_RATE_WINDOW`

## 6. API cache и безопасность

- кешируются только успешные GET ответы;
- cache key включает tenant/user/version;
- mutating запросы поднимают версии инвалидации.

Это снижает риск утечки stale cross-tenant данных.

## 7. SQL injection и query safety

Текущая кодовая база использует parameterized SQL через pgx:

- `QueryRow(ctx, q, arg1, arg2...)`
- `Query(ctx, q, args...)`

Опасные string-concat SQL из user input в репозиториях не используются.

Отдельно для inbound SQL connector:

- есть проверка read-only запроса перед выполнением.

## 8. Файлы и артефакты

Для загрузки вложений:

- размер ограничивается (`ARTIFACTS_MAX_UPLOAD_MB`);
- filename sanitize;
- checksum SHA256;
- payload в S3, не в БД;
- скачивание через presigned URL с TTL.

## 9. HTTP hardening

Применяются security headers middleware.

## 10. Audit trail

Ключевые действия логируются в `audit_logs`:

- auth события
- операции с alert/case/workspace
- connector действия
- AI analysis события

## 11. Риски и зоны усиления

Текущие зоны для усиления в production:

- добавить MFA enforcement policy;
- включить signed/encrypted audit export;
- ввести centralized secret manager;
- усилить outbound connector secret masking;
- добавить policy-level validation для AI prompts/outputs.
