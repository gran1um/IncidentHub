# Backend: архитектура и доменные модули

## 1. Стек и принципы

- Go backend (echo v5)
- PostgreSQL через `pgxpool`
- Redis (`go-redis`)
- Elasticsearch (`go-elasticsearch/v8`)
- S3 storage (`minio-go`)
- Kafka (`segmentio/kafka-go`) в connectors + `confluent-kafka-go` для async CRUD и notification delivery queue
- Логирование/трейсинг: anyinfra `logster`/`tracer` (как в alerter)

Принципы:

- stateless API-инстанс;
- tenant-scoped доступ;
- явное разделение handler/repository/service;
- forum proxy с pluggable outbound drivers;
- retrieval-augmented AI service.

## 2. Инициализация приложения

`/backend/cmd/api/main.go`:

1. загрузка ENV config;
2. init runtime (logger/tracing/metrics);
3. init PostgreSQL pool + миграции;
4. init Redis, Elasticsearch, S3;
5. создание repositories;
6. создание services (`outbound`, `forumproxy`, `ai`, async ops kafka queue, notification delivery queue, service alert evaluator, ai-agent queue repository);
7. запуск async ops consumer, notifications consumer, service alert evaluator и background AI agent queue worker;
8. bootstrap platform admin;
9. запуск HTTP server.

## 3. HTTP server и middleware pipeline

`/backend/internal/api/server.go` + `handler.go`

Глобально:

- panic recover;
- request ID;
- CORS;
- security headers;
- metrics middleware.

Auth + cache pipeline:

- `JWTAuth` для authed routes (JWT + API access token fallback);
- `APITokenAccessControl` для scope/full-access enforcement и запрета user-account endpoints;
- `APICacheInvalidation` (для mutating методов);
- `APICache` (для GET методов).

AD/LDAP аутентификация:

- `POST /api/v1/auth/login` поддерживает `ldap_enabled` пользователей;
- flow: `ldapsearch` (по `LDAP_BASE_DN` + `LDAP_USER_FILTER`) -> bind пользователя через `ldapwhoami`;
- если AD/LDAP проверка неуспешна, логин отклоняется `401`.

Tenant pipeline:

- `ResolveTenant` (membership check + tenant context);
- `RequirePlatformAdmin` / `RequireTenantAdminOrPlatformAdmin` / `RequireAnalystOrHigher`.

Разграничение доступа к экранам/кейсам:

- backend:
  - role-gates на endpoint-ах (`RequireAnalystOrHigher`, `RequireTenantAdminOrPlatformAdmin`);
  - tenant-изоляция (`ResolveTenant`);
  - case-level ограничение через SOC policy (`allowed_case_tags`, `max_cases_in_work`);
- frontend:
  - роуты investigations (`alerts/cases/forum/templates/antifraud/activity/workflow`) доступны только analyst+.

## 4. Структура API handlers

Файлы handlers разнесены по доменам:

- `handler_auth_system.go`
- `handler_auth_sessions.go`
- `handler_tenants_users.go`
- `handler_alerts_cases.go`
- `handler_case_ops.go`
- `handler_case_statuses.go`
- `handler_forum_catalog.go`
- `handler_antifraud_incidents.go`
- `handler_datasec_incidents.go`
- `handler_case_communications.go`
- `handler_connectors_ai_proxy_swagger.go`
- `handler_ai_case_analysis.go`
- `handler_notifications.go`
- `handler_workflows.go`
- `handler_system_runtime.go`

Это упрощает поддержку, обзор и тестирование.

## 5. Доменный слой (repository)

Репозитории в `/backend/internal/repository` покрывают:

- users, tenants, memberships;
- refresh tokens;
- alerts/cases/tasks;
- case cross-tenant shares (`case_tenant_shares`) for single-entity collaboration;
- observables/events/pages/attachments;
- catalog items;
- audits;
- connector ingest states;
- forum external bindings;
- connector recipient aliases;
- api access tokens;
- async operation tracker (`async_operations`);
- workflow runs (`workflow_runs`);
- telegram notification bots/settings;
- AI chats и case analyses;
- system stats/resources.

Все SQL-запросы параметризованы (`$1, $2...`) — без string concatenation для входных данных.

### 5.1 Cases list summary API

Для карточек списка кейсов добавлен tenant-scoped endpoint:

- `GET /api/v1/cases/summary?case_ids=<uuid,uuid,...>`

Реализация:

- handler: `handler_case_list_summary.go`;
- task summary: `TaskRepository.CountByCaseIDs` (open/closed/total + progress);
- responder/automation summary: `WorkflowRunRepository.CountStatusesByCaseIDs` (queued/running/completed/failed).

### 5.2 Activity livestream API

Для live-ленты в UI добавлен tenant-scoped endpoint:

- `GET /api/v1/activity/livestream`

### 5.3 SOC access policy (tag segmentation + workload limit)

Tenant-scoped policy хранится в `catalog_items` (`kind=soc_access_policies`) и применяется в runtime:

- `allowed_case_tags`:
  - для analyst-role ограничивает доступ к кейсам по `case_meta.tags` (list/get и case-bound endpoints через `resolveCaseInTenant`);
  - фильтрация выполняется на backend, не зависит от frontend.
- `max_cases_in_work`:
  - ограничивает назначение кейсов на аналитика при `POST /cases`, `PATCH /cases/:id`, `POST /alerts/bulk/create-case`;
  - проверка основана на количестве open-status кейсов у assignee (tenant case statuses, где `is_closed=false`).

Ключевые файлы:

- `/backend/internal/api/handler_soc_access_policy.go`
- `/backend/internal/api/handler_alerts_cases.go`
- `/backend/internal/repository/case_repository.go`
- `/backend/internal/repository/case_repository_policy.go`

Поддерживаемые query-параметры:

- `limit` (1..500);
- `types` (`case,alert,task`);
- `q` (поиск по title/description/status/source/id);
- `assignee_id` (UUID);
- `since` (RFC3339).

Реализация:

- handler: `handler_activity_livestream.go`;
- источник данных: `CaseRepository`, `AlertRepository`, `TaskRepository`;
- выдача нормализована в единый список `activityLivestreamItem` и сортируется по `updated_at` (desc).

### 5.4 Dashboard metrics API + custom metric evaluation

Для блока метрик добавлен handler:

- `handler_dashboard_metrics.go`

Новые tenant-scoped endpoints:

- `GET /api/v1/dashboard/metrics`
- `GET /api/v1/dashboard/metrics/custom`
- `POST /api/v1/dashboard/metrics/custom`
- `PATCH /api/v1/dashboard/metrics/custom/:metricID`
- `DELETE /api/v1/dashboard/metrics/custom/:metricID`

Репозиторный слой:

- `SystemRepository.TenantDashboardMetrics(...)`
- `SystemRepository.EvaluateDashboardCustomMetric(...)`

Что считает backend:

- open cases/open alerts;
- overdue cases counter (threshold configurable через query `overdue_minutes`);
- SLA-by-severity (open/resolved/breached/avg resolution);
- resolved cases by analyst;
- case/alert breakdown по status/category.

Custom metric definitions хранятся tenant-scoped в `catalog_items` (`kind=dashboard_metrics`), а вычисление выполняется на SQL-агрегатах по `cases/alerts` + `case_meta.category`.

## 6. Коммуникации и фоновые процессы

### 6.1 Connector runtime (v1)

В публичном продукте используется outbound-only runtime:

- операторский UI и tenant-scoped API surface работают только с `outbound_connectors`;
- execution visibility идет через durable Connector Hub (`connector_executions`, attempts/events, drawer/timeline в UI);
- connector methods являются единым контрактом для analyst/agent actions по observables, cases и alerts.

### 6.2 Outbound service

`/backend/internal/connectors/outbound/service.go`

- выбирает driver по типу канала;
- поддерживает send/poll;
- драйверы: mock, webhook, telegram, email, time, kafka, redis;
- поддерживает connector-level auth/config (token/basic/api key/headers) и tenant-scoped secrets/config из `catalog`;
- используется как транспорт для case communications (`send` + `sync`) и для workflow connector actions.

### 6.3 Forum proxy

`/backend/internal/forumproxy/service.go`

- связывает thread <-> connector через `forum_external_bindings`;
- сохраняет `conversation_id`, `cursor`, metadata;
- резолвит target/username -> recipient id через `connector_recipient_aliases` (для Telegram и будущих каналов);
- обеспечивает двусторонний обмен сообщениями.

### 6.4 Case communications

`/backend/internal/api/handler_case_communications.go`

- отдельный кейсовый API для внешних диалогов;
- thread/message хранение в `catalog_items`;
- outbound send/sync через общий `forumproxy.Service`;
- поддержка participant metadata для Telegram/Email/Time;
- поддержка шаблонов сообщений (`catalog kind=communication_templates`) через `template_id` + `template_vars`;
- sync email thread может фильтровать входящие письма по `subject` (case-bound mailbox pull);
- tenant-изоляция и case-scope проверка для всех операций.
- `resolveCaseInTenant` учитывает `case_tenant_shares`: shared tenant получает доступ к тому же case record (без копирования).

### 6.5 Workflow runtime

Workflow runtime реализован в:

- `/backend/internal/workflow/runtime.go`
- `/backend/internal/workflow/definition.go`
- `/backend/internal/workflow/nodes/*`

API-слой выполнения:

- `POST /api/v1/workflows/:workflowID/run` — production run сохраненного graph definition
- `POST /api/v1/workflows/:workflowID/test-run` — test-run с optional runtime override definition
- `GET /api/v1/workflows/:workflowID/runs` — история запусков (`workflow_runs`)

Core pipeline:

1. загрузка/нормализация definition (`nodes`, `edges`, `entryNodeId`);
2. создание `workflow_runs` записи со status `running`;
3. пошаговый traversal + dispatch node executor по `node.type`;
4. финализация run (`success|failed`) и сохранение result/error;
5. retention history до последних 1000 запусков.

### 6.6 Anti-fraud incident runtime

`/backend/internal/api/handler_antifraud_incidents.go`

- инциденты вынесены в отдельный tenant-scoped kind `catalog/antifraud_incidents` (не смешиваются с `antifraud_events`);
- rules читаются из `catalog/antifraud_incident_rules`, поддерживаются conditions:
  - `channel`,
  - `recipient_type`,
  - `min_records`,
  - `min_critical_accesses`,
  - `min_amount`,
  - `risk_levels`,
  - `event_statuses`;
- routing API:
  - `POST /api/v1/antifraud/incidents/route` (bulk routing events -> incidents);
  - auto-routing также срабатывает на create/update `catalog/antifraud_events`;
- investigation API:
  - `GET /api/v1/antifraud/incidents` (filters + pagination + embedded actions history),
  - `POST /api/v1/antifraud/incidents/:incidentID/actions` (runtime action execution),
  - `PATCH /api/v1/antifraud/incidents/:incidentID/status` (status change + optional auto `send_closure_email`);
- actions пишутся в `catalog/antifraud_incident_actions`, поддерживаются типы:
  - `send_initial_email`, `send_closure_email`,
  - `delete_client_file`, `delete_time_file`, `delete_mail_file`, `delete_data_file`, `delete_share_file`;
- исполнение действий идет через общий outbound/forumproxy runtime (tenant-scoped connector resolution, delivery status persisted).

### 6.7 DataSec incident runtime (Incident-as-a-Service)

`/backend/internal/api/handler_datasec_incidents.go`

- tenant-scoped runtime для инцидентов DataSec (`catalog/datasec_incidents`);
- lifecycle endpoints:
  - `GET /api/v1/datasec/incidents` (filters + pagination + embedded actions);
  - `POST /api/v1/datasec/incidents` (registration, обязательные поля интеграции: `mdm`, `scenario_id`, `incident_date`);
  - `POST /api/v1/datasec/incidents/:incidentID/ping`;
  - `POST /api/v1/datasec/incidents/:incidentID/escalate`;
  - `PATCH /api/v1/datasec/incidents/:incidentID/status` (включая закрытие);
  - `POST /api/v1/datasec/incidents/:incidentID/measures` (mitigation actions);
  - `GET /api/v1/datasec/incidents/metrics` (status/created/closed/measures/counts);
  - `GET /api/v1/datasec/faq` (первая линия DLP/Detection, tenant/global catalog fallback).
- поддерживаемые меры (`measure_type`):
  - `training_assignment`, `access_review`, `selective_awareness`,
  - `disciplinary_action`, `dismissal`, `fine`, `yellow_card`, `red_card`,
  - `block_account`, `block_device`.
- все действия пишутся в tenant-scoped `catalog/datasec_incident_actions`.

Текущие built-in executable nodes:

- trigger: `trigger`
- control flow: `condition`, `switch`, `delay`, `scheduler`
- data processing: `transform`, `aggregate`
- connector actions: `telegram_send`
- data/integration: `postgres_query`, `redis`, `cassandra_query`, `s3_object`
- compute: `python_code`

### 6.8 Notification delivery queue

`/backend/internal/api/notifications_delivery_kafka.go`

- enqueue события уведомлений при `catalog kind=notifications`;
- producer/consumer в Kafka topic `NOTIFICATIONS_KAFKA_TOPIC`;
- retry-policy: exponential backoff + jitter, `NOTIFICATIONS_RETRY_MAX_ATTEMPTS`;
- DLQ fallback в `NOTIFICATIONS_KAFKA_DLQ_TOPIC`;
- перед отправкой учитываются tenant-scoped `user_notification_settings` и выбранный админом `telegram_notification_bot`;
- tenant admin/platform admin управляет `user_notification_settings` для всех пользователей своего тенанта через `/api/v1/admin/notification-settings`;
- фактическая доставка выполняется через общий `outbound.Service` (telegram driver).

### 6.8 Service alert evaluator

`/backend/internal/api/service_alerts.go`

- периодически читает tenant-scoped rules из `catalog/service_alert_rules`;
- поддерживает правила `low_rps` и `high_latency` по in-process module operation метрикам;
- при breach создает `catalog kind=notifications` и enqueue в notification Kafka queue;
- учитывает cooldown per rule, чтобы исключить постоянный spam;
- доставка в Telegram выполняется штатным notification consumer только для пользователей с включенным telegram delivery.

## 7. AI service

`/backend/internal/ai/service.go`

- retrieval в Elasticsearch с tenant контекстом;
- LLM provider (Ollama) через chat API;
- режимы: общий AI ask и case analysis;
- fallback ответы, если LLM недоступна;
- сохранение истории в PostgreSQL.

### 7.1 AI Agent Queue dispatcher

`/backend/internal/api/handler_ai_agents_queue.go`

- очередь в PostgreSQL (`ai_agent_queue_events`) для новых `alerts/cases`;
- enqueue из sync и async create-paths (`handler_alerts_cases.go`, `handler_case_escalation.go`, `async_ops_kafka.go`);
- background worker matсhing `catalog(kind=ai_agents)` по `target_types/case_tags/alert_sources`;
- stage-based execution (`investigation_plan`) + enrichment connectors;
- запись результата в `catalog(kind=ai_agent_runs)`;
- опциональные post-actions: case tags, auto-close, notify connectors.

## 8. Async CRUD (Kafka)

`/backend/internal/api/async_ops_kafka.go`

- async create/delete для alerts/cases;
- async create/update/delete для alerts/cases;
- async copy для cases (`POST /api/v1/cases/:caseID/copy`);
- единый контракт `202 queued` + `operation_id/resource/resource_id`;
- состояние операции пишется в PostgreSQL (`async_operations`);
- polling статуса через `GET /api/v1/operations/:operationID`;
- retry-policy: exponential backoff + jitter, `ASYNC_OPS_RETRY_MAX_ATTEMPTS`;
- после исчерпания попыток сообщение уходит в DLQ (`ASYNC_OPS_KAFKA_DLQ_TOPIC`) и статус становится `failed`;
- успешные delete операции синхронизируют search index через `DeleteDocument`.
- `PATCH /api/v1/cases/:caseID` поддерживает optimistic concurrency (`expected_updated_at`) для защиты от stale-update при параллельной работе аналитиков.

## 9. Системные endpoints

- `/healthz` — легкая проверка API
- `/api/v1/system/health` — модульное здоровье (`api`, `postgres`, `redis`, `elasticsearch`, `s3`, `ai_model`) с timeout 5 секунд на модуль
- `/api/v1/system/resources` — host/tenant/system срез для админ-панели
- `/metrics` — Prometheus метрики

## 10. Горизонтальное масштабирование backend

Текущая архитектура backend-инстанса позволяет scale-out:

- сессии и refresh-токены в PostgreSQL;
- rate-limit в Redis;
- API cache в Redis;
- async operation state в PostgreSQL;
- файловое хранилище вне инстанса (S3/MinIO).

Практический запуск:

- `docker compose up -d --scale backend=3`

Критические caveats для масштаба:

- `RequestsLast24h` в метриках сервиса считается в памяти конкретного инстанса (не агрегируется между инстансами);
- чтобы видеть глобальные API request trends при scale-out, использовать Prometheus queries по `/metrics`, а не in-process счетчик.
