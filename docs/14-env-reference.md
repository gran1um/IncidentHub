# ENV-конфигурация (детальный справочник)

Источник актуальных ключей:

- `./.env.example`
- `./backend/internal/config/config.go`

## 1. Общие

- `ENV` — окружение (`dev/test/prod`)
- `COMPOSE_PROJECT_NAME` — фиксирует имя docker compose проекта (стабильные имена контейнеров/томов между перезапусками)

## 2. HTTP

- `HTTP_ADDR`
- `HTTP_CORS_ALLOW_ORIGINS`
- `HTTP_SECURE_COOKIES`
- `HTTP_READ_TIMEOUT`
- `HTTP_WRITE_TIMEOUT`
- `HTTP_SHUTDOWN_TIMEOUT`

Рекомендация для локального Ollama:

- `HTTP_WRITE_TIMEOUT` держать >= `AI_TIMEOUT` + запас (например `60s`).

## 3. Auth

- `AUTH_JWT_SECRET`
- `AUTH_ISSUER`
- `AUTH_ACCESS_TTL`
- `AUTH_REFRESH_TTL` (по умолчанию `24h`)
- `AUTH_REFRESH_COOKIE_NAME`

## 4. Security

- `SEC_PASSWORD_MIN_LENGTH`
- `SEC_LOGIN_RATE_LIMIT`
- `SEC_LOGIN_RATE_WINDOW`

## 5. Artifacts

- `ARTIFACTS_MAX_UPLOAD_MB`
- `ARTIFACTS_PRESIGN_TTL`

## 6. PostgreSQL

- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`
- `POSTGRES_SSLMODE`
- `POSTGRES_MAX_CONNS`
- `POSTGRES_MIN_CONNS`

## 7. Redis

- `REDIS_ADDR`
- `REDIS_USERNAME`
- `REDIS_PASSWORD`
- `REDIS_DB`

## 8. API cache

- `API_CACHE_ENABLED`
- `API_CACHE_TTL`
- `API_CACHE_MAX_BODY_BYTES`

## 9. S3

- `S3_ENABLED`
- `S3_ENDPOINT`
- `S3_PUBLIC_ENDPOINT`
- `S3_REGION`
- `S3_BUCKET`
- `S3_ACCESS_KEY`
- `S3_SECRET_KEY`
- `S3_USE_SSL`
- `S3_AUTO_CREATE_BUCKET`

## 10. Elasticsearch

- `ELASTIC_ENABLED`
- `ELASTIC_ADDRESSES`
- `ELASTIC_USERNAME`
- `ELASTIC_PASSWORD`
- `ELASTIC_INDEX_PREFIX`

## 11. LDAP

- `LDAP_ENABLED`
- `LDAP_URL`
- `LDAP_BASE_DN`
- `LDAP_BIND_DN`
- `LDAP_BIND_PASSWORD`
- `LDAP_USER_FILTER`

Примечание:

- backend LDAP flow использует утилиты `ldapsearch` и `ldapwhoami` (должны быть установлены в runtime).

## 12. Bootstrap

- `BOOTSTRAP_ENABLED`
- `BOOTSTRAP_PLATFORM_ADMIN_EMAIL`
- `BOOTSTRAP_PLATFORM_ADMIN_USERNAME`
- `BOOTSTRAP_PLATFORM_ADMIN_PASSWORD`
- `BOOTSTRAP_PLATFORM_ADMIN_FULL_NAME`

## 13. Logging/Tracing/Metrics

- `LOGGER_GROUP`
- `LOGGER_SYSTEM`
- `LOG_LEVEL`
- `TRACE_ENABLED`
- `TRACE_TENANT`
- `TRACE_SERVICE_KEY`
- `METRICS_NAMESPACE`

## 14. Connectors

Outbound:

- `CONNECTORS_OUTBOUND_HTTP_TIMEOUT`
- `CONNECTORS_OUTBOUND_TELEGRAM_API_BASE_URL` — base URL for Telegram Bot API validation/delivery (default `https://api.telegram.org`)

Примечание:

- публичный UI/API surface в v1 работает только с outbound connectors;
- connector env используется runtime-слоем Connector Hub, forum proxy, notifications и workflow nodes.

## 15. Workflow Runtime (Native)

- `WORKFLOW_ENGINE` (`internal`)

Назначение:

- используется только встроенный runtime в backend (`internal/workflow/*`);
- Workflow Studio работает в native-режиме (список workflow + editor canvas + run history);
- AI-агент в Workflow Studio генерирует workflow JSON через `POST /api/v1/ai/ask` и применяет драфт в canvas;
- built-in security nodes доступны в library: `ioc_extract`, `watchlist_match`, `mitre_map`, `risk_score`, `containment_decision`.

## 16. Async operations (Kafka queue)

- `ASYNC_OPS_ENABLED`
- `ASYNC_OPS_KAFKA_BROKERS`
- `ASYNC_OPS_KAFKA_TOPIC`
- `ASYNC_OPS_KAFKA_DLQ_TOPIC`
- `ASYNC_OPS_KAFKA_GROUP_ID`
- `ASYNC_OPS_PRODUCE_TIMEOUT`
- `ASYNC_OPS_POLL_TIMEOUT`
- `ASYNC_OPS_RETRY_MAX_ATTEMPTS`
- `ASYNC_OPS_RETRY_BASE_BACKOFF`
- `ASYNC_OPS_RETRY_MAX_BACKOFF`

Назначение:

- включает асинхронную обработку `POST/DELETE` для alerts/cases;
- producer/consumer работают через Kafka topic, заданный в `ASYNC_OPS_KAFKA_TOPIC`.

## 17. Notification delivery (Kafka)

- `NOTIFICATIONS_KAFKA_ENABLED`
- `NOTIFICATIONS_KAFKA_BROKERS`
- `NOTIFICATIONS_KAFKA_TOPIC`
- `NOTIFICATIONS_KAFKA_DLQ_TOPIC`
- `NOTIFICATIONS_KAFKA_GROUP_ID`
- `NOTIFICATIONS_PRODUCE_TIMEOUT`
- `NOTIFICATIONS_POLL_TIMEOUT`
- `NOTIFICATIONS_RETRY_MAX_ATTEMPTS`
- `NOTIFICATIONS_RETRY_BASE_BACKOFF`
- `NOTIFICATIONS_RETRY_MAX_BACKOFF`

Назначение:

- событие уведомления создается в catalog (`kind=notifications`);
- backend enqueue в Kafka (`NOTIFICATIONS_KAFKA_TOPIC`);
- consumer читает пользовательские настройки доставки и отправляет в Telegram через выбранного admin-configured бота.

### 17.1 Service degradation alerts (RPS/latency -> Telegram)

- `SERVICE_ALERTING_ENABLED`
- `SERVICE_ALERTING_EVALUATION_INTERVAL`
- `SERVICE_ALERTING_DEFAULT_WINDOW`
- `SERVICE_ALERTING_DEFAULT_COOLDOWN`
- `SERVICE_ALERTING_RULES_LIMIT`
- `SERVICE_ALERTING_RECIPIENTS_LIMIT`

Назначение:

- включает фоновый evaluator правил `catalog/service_alert_rules` (tenant-scoped);
- evaluator использует in-process module metrics (`incidenthub_module_operations_total`, `incidenthub_module_operation_duration_seconds`);
- при breach правила создает `catalog/notifications` и отправляет событие в Kafka notification queue;
- Telegram доставка выполняется только для пользователей с `delivery_enabled=true`, `delivery_channel=telegram` и валидной bot/chat конфигурацией.

### 17.2 Local operator shortcuts (manual smoke only)

- `TELEGRAM_ALERT_BOT_TOKEN`
- `TELEGRAM_ALERT_USER_ID`
- `TELEGRAM_ALERT_CHAT_ID`

Это локальные переменные для ручных smoke-скриптов/`curl` в dev-окружении.
Backend их не читает как runtime-конфиг доставки; рабочая конфигурация задается через API (`/admin/notification-bots`, `/me/notification-settings`).

## 18. AI

- `AI_ENABLED`
- `AI_PROVIDER`
- `AI_ENDPOINT`
- `AI_API_KEY`
- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `AI_MODEL`
- `AI_TIMEOUT`
- `AI_TOP_K`
- `AI_MAX_CONTEXT_CHARS`
- `AI_HISTORY_MESSAGES`
- `AI_MCP_ENABLED`
- `AI_MCP_TOOLS_ALLOWLIST`
- `AI_MCP_PER_TOOL_LIMIT`
- `AI_SYSTEM_PROMPT`
- `AI_AGENT_QUEUE_ENABLED`
- `AI_AGENT_QUEUE_POLL_INTERVAL`
- `AI_AGENT_QUEUE_BATCH_SIZE`
- `AI_AGENT_QUEUE_RUN_TIMEOUT`

Рекомендации:

- Для OpenAI-compatible шлюза используйте `AI_PROVIDER=openai-compatible`.
- `AI_ENDPOINT` поддерживает два формата:
  - base URL (например `http://host:8001/v1`) — backend сам добавит `/chat/completions`;
  - полный chat URL (например `http://host:8001/v1/chat/completions`).
- `OPENAI_API_KEY`/`AI_API_KEY` можно оставлять пустыми для локальных gateway без auth.
- Для моделей класса Qwen/Llama обычно безопаснее `AI_TIMEOUT=60s` (слишком короткие значения дают ложные 502 по timeout).
- `AI_MCP_ENABLED=true` включает read-only tenant-scoped MCP-слой для загрузки PostgreSQL контекста в AI.
- `AI_MCP_TOOLS_ALLOWLIST` ограничивает инструменты (поддержка: `open_case_statuses`, `cases_in_work`, `recent_cases`, `recent_alerts`, `dashboard_stats`).
- `AI_MCP_PER_TOOL_LIMIT` ограничивает объем выборки на инструмент (например для `recent_cases`/`recent_alerts`).
- `AI_AGENT_QUEUE_ENABLED` включает background-dispatch для auto-run AI-агентов на созданных alerts/cases.
- `AI_AGENT_QUEUE_POLL_INTERVAL` задает период чтения очереди (`ai_agent_queue_events`).
- `AI_AGENT_QUEUE_BATCH_SIZE` ограничивает число queue events за один poll-цикл.
- `AI_AGENT_QUEUE_RUN_TIMEOUT` задает timeout на обработку одного queue event.
- Если в ответах часто приходит `model=sec-fallback-rag`, текущая LLM деградирует: увеличьте timeout/ресурсы или переключитесь на более сильную модель.

## 19. Frontend

- `VITE_API_URL`
- `VITE_SUPPORT_URL`

## 20. Infra-override ключи

- `POSTGRES_IMAGE`
- `ELASTIC_IMAGE`
- `ELASTIC_PLATFORM`
- `GRAFANA_ADMIN_USER`
- `GRAFANA_ADMIN_PASSWORD`

## 21. Test DB safety

- `TEST_DATABASE_URL` — отдельная база для интеграционных тестов (только `*_test`)
- `INCIDENTHUB_TEST_DATABASE_URL` — fallback для тестов
- `INCIDENTHUB_ALLOW_UNSAFE_TEST_DB` — аварийный override защиты (не использовать в обычной разработке)

## 22. Практические правила управления env

- Никогда не хранить production secrets в git.
- Для каждого окружения использовать отдельный `.env` + secret manager.
- При ротации `AUTH_JWT_SECRET` учитывать сессионное воздействие.
- Перед включением `S3_ENABLED=true` проверять bucket ACL/policy.
- Для multi-node backend настроить одинаковые cache/rate-limit env.
