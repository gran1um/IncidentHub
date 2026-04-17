# API: карта маршрутов и контрактов

Базовый префикс: `/api/v1`

## 1. Аутентификация и сессии

### Public

- `POST /auth/login`
- `POST /auth/refresh`

### Auth required

- `POST /auth/logout`
- `GET /auth/sessions`
- `POST /auth/sessions/revoke-others`

### Tenant admin API tokens

Tenant-scoped:

- `GET /admin/api-tokens` (tenant admin/platform admin)
- `POST /admin/api-tokens` (tenant admin/platform admin)
- `DELETE /admin/api-tokens/:tokenID` (tenant admin/platform admin)

## 2. Пользователи и тенанты

- `GET /me`
- `GET /tenants`
- `POST /tenants` (platform admin)
- `PATCH /tenants/:tenantID` (platform admin)

- `GET /users`
- `GET /users/:id`
- `PATCH /users/:id`
- `POST /users` (tenant admin or platform admin + tenant context)
- `POST /users/:id/media/:kind/upload` (`kind=avatar|cover`)

XP/performance:

- `GET /users/:id/experience-events`
- `GET /users/:id/performance`
- `POST /users/:id/experience-awards` (tenant admin or platform admin + tenant context)

`GET /users/:id/experience-events`:

- query params: `limit` (1..200, default 50), `offset` (>=0, default 0);
- response: массив событий (descending by `created_at`), пригоден для lazy/infinite loading на UI.

Tenant-scoped:

- `GET /tenant-users`

Notification delivery settings (tenant-scoped):

- `GET /notification-bots` (analyst+; список включенных ботов текущего тенанта)
- `GET /me/notification-settings` (analyst+)
- `PUT /me/notification-settings` (analyst+)

Notification bot admin endpoints (tenant-scoped):

- `GET /admin/notification-bots` (tenant admin/platform admin)
- `POST /admin/notification-bots` (tenant admin/platform admin)
- `PATCH /admin/notification-bots/:botID` (tenant admin/platform admin)
- `DELETE /admin/notification-bots/:botID` (tenant admin/platform admin)
- `GET /admin/notification-settings` (tenant admin/platform admin; список user delivery settings по тенанту)
- `PUT /admin/notification-settings/:userID` (tenant admin/platform admin; upsert user delivery settings в рамках тенанта)

Примечания:

- смена пароля для своего пользователя требует `current_password` в payload;
- `experience-awards` требует `description` и `points > 0`.
- API access tokens не могут вызывать user-account endpoints (включая `POST /auth/logout`, профиль/пароль/сессии/списки пользователей), исключение: `POST /users/:id/experience-awards`.

## 3. Alerts

Tenant-scoped:

- `GET /alerts`
- `GET /alerts/:alertID`
- `POST /alerts`
- `PATCH /alerts/:alertID`
- `DELETE /alerts/:alertID`
- `POST /alerts/bulk/link-case`
- `POST /alerts/bulk/create-case`

`GET /alerts` поддерживает два режима:

- legacy: массив (`limit`, `offset`);
- pagination: объект `{ items, page, page_size, total, total_pages }` при передаче `page` и/или `page_size` (`page_size` допускает `10|30|50|100`).

## 4. Cases, статусы и dashboard

Tenant-scoped:

- `GET /dashboard/stats`
- `GET /dashboard/metrics`
- `GET /dashboard/metrics/custom`
- `POST /dashboard/metrics/custom`
- `PATCH /dashboard/metrics/custom/:metricID`
- `DELETE /dashboard/metrics/custom/:metricID`
- `GET /duty/overview`
- `GET /activity/livestream`
- `GET /case-statuses`
- `PUT /case-statuses` (tenant admin/platform admin)

- `GET /cases`
- `GET /cases/summary`
- `POST /cases`
- `GET /cases/:caseID`
- `POST /cases/:caseID/copy`
- `GET /cases/:caseID/related`
- `PATCH /cases/:caseID`
- `DELETE /cases/:caseID`
- `GET /cases/:caseID/shares`
- `POST /cases/:caseID/share`

`GET /cases` поддерживает два режима:

- legacy: массив (`limit`, `offset`);
- pagination: объект `{ items, page, page_size, total, total_pages }` при передаче `page` и/или `page_size` (`page_size` допускает `10|30|50|100`).
- sorting:
  - `sort_by`: `updated_at|created_at|severity|status|title|case_number` или `cf:<custom_field_key>`;
  - `sort_order`: `asc|desc`;
- при включенной policy `catalog/soc_access_policies.allowed_case_tags` для analyst-role список фильтруется по пересечению с `case_meta.tags`.

Workload guard:

- при включенной policy `catalog/soc_access_policies.max_cases_in_work` backend отклоняет назначения, если у assignee достигнут лимит активных кейсов (`409`).
- проверка применяется в:
  - `POST /cases`
  - `PATCH /cases/:caseID`
  - `POST /alerts/bulk/create-case`

`GET /cases/summary`:

- принимает `case_ids` (csv UUID);
- возвращает summary-модель на кейс:
  - `open_tasks`, `closed_tasks`, `total_tasks`, `task_progress_percent`;
  - `responder_statuses.{queued,running,completed,failed}`.

Cross-tenant collaboration:

- `POST /cases/:caseID/share` не создает копию; кейс становится доступен в target tenant как та же сущность;
- после share изменения из любого подключенного tenant применяются к одному case record;
- `GET /cases/:caseID/shares` возвращает список tenant-ов, подключенных к кейсу.

`GET /cases/:caseID/related`:

- поддерживает `link_by`:
  - `observables` (default) — корреляция по совпадающим observables (`type + value`);
  - `incident_type|source|severity|priority` — корреляция по связанному полю кейса;
- возвращает две выборки: `active_recent` (активные или недавние) и `all_time` (за весь период);
- возвращает `link_by`, `matched_observables` и/или `matched_fields` для каждого related case;
- учитывает tenant-access policy: кейсы вне доступа текущего tenant-а в результат не попадают.

`PATCH /cases/:caseID` (закрытие с апрувом):

- staged-close остается ролевым:
  - перевод в `review` для analyst требует назначения на другого активного analyst/admin;
  - финальное `status=closed` — только tenant admin/platform admin;
- поддержан approval-gate для финального закрытия:
  - конфиг в `case_meta.custom_fields`:
    - `closure_required_approvals` (int >= 0),
    - `closure_approver_ids` (csv/json-array UUID);
  - апрувы фиксируются через `POST /cases/:caseID/events` с `event_type=closure_approval`;
  - при нехватке апрувов или отсутствии обязательных апруверов `PATCH /cases/:caseID` возвращает `409`.
- поддерживает optimistic concurrency через `expected_updated_at` (RFC3339/RFC3339Nano):
  - если кейс был обновлен другим пользователем после загрузки формы, backend возвращает `409`;
  - клиенту нужно перечитать `GET /cases/:caseID` и повторить patch с актуальной версией.

`POST /cases/:caseID/copy`:

- создает новый кейс в том же tenant на основе существующего;
- по умолчанию генерирует новый `case_number`, сохраняет основные operational поля и assignee;
- в async-режиме возвращает `202 queued`, в sync-режиме `201 created`.

`GET /duty/overview`:

- возвращает `on_duty`, `current`, `next` + `meta` для графика дежурств;
- `at` (optional, RFC3339) задает точку времени для расчета текущих/следующих смен;
- `limit` (optional, default `2000`, max `5000`) ограничивает количество загружаемых shift-записей из `catalog/shifts`.

`GET /activity/livestream`:

- единый live-feed по `case|alert|task` для интерфейса;
- поддерживает:
  - `limit` (1..500),
  - `types` (`case,alert,task`),
  - `q` (текстовый фильтр),
  - `assignee_id` (UUID),
  - `since` (RFC3339);
- возвращает `{ items, generated_at }`, где `items` отсортированы по `updated_at desc`.

`GET /dashboard/metrics`:

- возвращает агрегированный metrics snapshot:
  - `open_cases`, `open_alerts`, `overdue_cases`, `overdue_threshold_m`;
  - `sla_by_severity[]`;
  - `resolved_by_analyst[]`;
  - `cases_by_status[]`, `cases_by_category[]`;
  - `alerts_by_status[]`, `alerts_by_category[]`;
  - `custom_metrics[]` (tenant-defined metrics с текущим value/error).
- query:
  - `overdue_minutes` (optional, >0; default `1440`).

`GET /dashboard/metrics/custom`:

- возвращает `{ items: [...] }` для tenant custom metrics.

`POST /dashboard/metrics/custom` и `PATCH /dashboard/metrics/custom/:metricID`:

- payload:
  - `name`, `description`,
  - `source` (`cases|alerts`),
  - `measure` (`count|avg_resolution_minutes|overdue_count`),
  - `enabled`,
  - `filters`:
    - `statuses[]`, `severities[]`, `categories[]`,
    - `created_from`, `created_to`, `closed_from`, `closed_to` (RFC3339/date),
    - `created_within_hours`, `closed_within_hours`,
    - `overdue_minutes`.
- ограничения:
  - `avg_resolution_minutes` и `overdue_count` доступны только для `source=cases`.

### 4.1 DataSec Incident-as-a-Service

Tenant-scoped:

- `GET /datasec/incidents`
- `POST /datasec/incidents`
- `POST /datasec/incidents/:incidentID/ping`
- `POST /datasec/incidents/:incidentID/escalate`
- `POST /datasec/incidents/:incidentID/measures`
- `PATCH /datasec/incidents/:incidentID/status`
- `GET /datasec/incidents/metrics`
- `GET /datasec/faq`

`POST /datasec/incidents`:

- обязательные поля: `mdm`, `scenario_id`, `incident_date`;
- дополнительные поля: `title`, `description`, `employment_state`, `severity`, `status`, `tags[]`, `owner_id`, `metadata`.

`POST /datasec/incidents/:incidentID/measures`:

- payload: `measure_type`, `note`, `metadata`;
- поддерживаемые `measure_type`:
  - `training_assignment`, `access_review`, `selective_awareness`,
  - `disciplinary_action`, `dismissal`, `fine`, `yellow_card`, `red_card`,
  - `block_account`, `block_device`.

`GET /datasec/incidents/metrics` возвращает:

- `total_incidents`, `open_incidents`, `closed_incidents`;
- `by_status` (карта статусов);
- `by_measure_type` (карта мер);
- `total_measures`, `oldest_open_days`.

## 5. Tasks

Tenant-scoped:

- `GET /tasks`
- `POST /tasks`
- `PATCH /tasks/:taskID`
- `DELETE /tasks/:taskID`

Payload fields (`POST/PATCH`):

- `case_id`, `title`, `description`, `status`, `assignee_id`;
- `due_date` (optional RFC3339):
  - `POST`: сохраняет дедлайн задачи;
  - `PATCH`: если передать `""`, дедлайн очищается.

## 6. Case workspace entities

Tenant-scoped:

- `GET /cases/:caseID/observables`
- `POST /cases/:caseID/observables`
- `PATCH /cases/:caseID/observables/:observableID`
- `DELETE /cases/:caseID/observables/:observableID`

- `GET /cases/:caseID/events`
- `POST /cases/:caseID/events`

`POST /cases/:caseID/events` now supports operational reminder events from case UI:

- `event_type=reminder`
- `metadata.trigger_at` (RFC3339), `metadata.offset_minutes`, `metadata.source`.

- `GET /cases/:caseID/pages`
- `POST /cases/:caseID/pages`

- `GET /cases/:caseID/attachments`
- `POST /cases/:caseID/attachments` (metadata only)
- `POST /cases/:caseID/attachments/upload` (multipart upload to S3)
- `GET /cases/:caseID/attachments/:attachmentID/download` (presigned URL)

## 7. Forum

Tenant-scoped:

- `GET /forum/threads`
- `GET /forum/threads/:threadID`
- `POST /forum/threads`
- `POST /forum/posts`
- `POST /forum/threads/:threadID/posts/upload`
- `POST /forum/threads/:threadID/proxy/send`
- `POST /forum/threads/:threadID/proxy/sync`

## 8. Case communications

Tenant-scoped:

- `GET /cases/:caseID/communications`
- `POST /cases/:caseID/communications`
- `GET /cases/:caseID/communications/:threadID`
- `POST /cases/:caseID/communications/:threadID/messages`
- `POST /cases/:caseID/communications/:threadID/sync`

Email templates and subject-aware sync:

- `POST /cases/:caseID/communications/:threadID/messages` поддерживает:
  - `template_id` — ID шаблона из `catalog/communication_templates`;
  - `template_vars` — объект переменных для рендера шаблона;
  - `subject` — явная тема письма;
  - `notify_managers` — включение manager-recipient flow.
- `POST /cases/:caseID/communications/:threadID/sync` поддерживает `subject`:
  - backend подтягивает из почтового ящика только сообщения, совпадающие по теме.

Telegram note for send endpoint:

- для прямой доставки используйте `participant.chat_id` (или `target`, который резолвится в alias-таблице);
- `participant.id` не интерпретируется как Telegram chat id;
- если получатель не резолвится, send endpoint отвечает `502`.

## 9. Комментарии

Tenant-scoped:

- `GET /case-comments`
- `POST /case-comments`

## 10. Communication connectors (Telegram/Email/Time)

Tenant-scoped:

- `GET /communications/connectors` (analyst+)

Примечание:

- connector management в v1 доступен через tenant-scoped `catalog` API:
  - `GET|POST|PATCH|DELETE /api/v1/catalog/outbound_connectors`
  - `GET|POST|PATCH|DELETE /api/v1/catalog/connector_methods`
- UI-конфигурация выполняется через `Administration -> Connectors`.
- в case communications поддерживаются каналы `telegram`, `email`, `time` (send + sync).
- endpoint отдает только поддерживаемые каналы для case communications (`telegram`, `email`, `time`), остальные outbound connectors фильтруются.

## 11. AI

Tenant-scoped:

- `GET /ai/sessions`
- `POST /ai/sessions`
- `DELETE /ai/sessions/:sessionID`
- `PATCH /ai/sessions/reorder`
- `GET /ai/session`
- `GET /ai/messages`
- `POST /ai/ask`
- `GET /ai/agents/entities/:entityType/:entityID`
- `GET /ai/agents/ops/overview` (tenant admin/platform admin)
- `POST /ai/agents/ops/events/:eventID/restart` (tenant admin/platform admin)
- `POST /ai/agents/ops/events/:eventID/close` (tenant admin/platform admin)
- `POST /ai/agents/ops/workloads/:workloadID/restart` (tenant admin/platform admin)
- `POST /ai/agents/ops/workloads/:workloadID/close` (tenant admin/platform admin)
- `POST /ai/agents/:agentID/run` (tenant admin/platform admin)
- `GET /ai/agents/:agentID/runs` (tenant admin/platform admin)
- `GET /cases/:caseID/ai/analyses`
- `POST /cases/:caseID/ai/analyze`

Примечания:

- конфигурации AI-агентов хранятся в generic catalog как `kind=ai_agents`; runs хранятся как `kind=ai_agent_runs`;
- `GET /ai/agents/entities/:entityType/:entityID` отдает queue/workload trace для case или alert и поддерживает `limit` (`1..20`, default `8`); workload snapshot включает `run.result.stage_timeline[]` и `run.result.connector_timeline[]`;
- manual run `POST /ai/agents/:agentID/run` поддерживает `case_ids[]`, `alert_ids[]`, `max_cases`, `dry_run`, `language`;
- workload-level restart/close предпочтительнее event-level controls, если нужно перезапустить только конкретный агент внутри queue event.

## 12. Workflow runtime

Tenant-scoped:

- `POST /workflows/:workflowID/run` — manual production run сохраненного workflow (аналитик+)
- `POST /workflows/:workflowID/test-run` — manual test-run сохраненного workflow (аналитик+)
- `GET /workflows/:workflowID/runs?limit=<n>` — история запусков workflow (аналитик+)

Примечания:

- `run` всегда исполняет сохраненную в catalog definition и не принимает runtime override;
- `workflowID` резолвится только в `catalog/workflows`;
- test-run выполняет реальный internal runtime, а не симуляцию обхода;
- `definition` можно передать прямо в запросе (без сохранения драфта в catalog);
- в истории запусков trigger различается: `manual` для `/run`, `manual_test` для `/test-run`;
- история хранит статус (`success|failed|running|queued`), длительность, summary/error, вход и результат исполнения;
- retention: последние `1000` запусков на workflow.
- генерация workflow через AI выполняется через `POST /ai/ask` (Workflow Studio AI Agent отправляет structured prompt и парсит JSON-ответ в draft).

Текущие built-in executable node types:

- `trigger`
- `condition`
- `switch`
- `delay`
- `scheduler`
- `transform`
- `aggregate`
- `ioc_extract`
- `watchlist_match`
- `mitre_map`
- `risk_score`
- `containment_decision`
- `redis`
- `postgres_query` (read-only SQL)
- `cassandra_query` (HTTP Cassandra gateway request)
- `telegram_send` (через outbound connector channel=telegram)
- `python_code`
- `s3_object`

## 13. Catalog

Tenant-scoped:

- `GET /catalog/:kind`
- `POST /catalog/:kind`
- `PATCH /catalog/:kind/:itemID`
- `DELETE /catalog/:kind/:itemID`

Через `catalog_items` реализованы:

- templates, case_templates
- observable_types
- achievements
- automations
- workflows (visual workflow graph JSON: `nodes`, `edges`, `entryNodeId`)
- antifraud_events (`external_id`, `title`, `subject`, `status`, `risk_level`, `amount`, `currency`, `event_time`, `tags`, `payload`)
- antifraud_incident_rules (`conditions`, `default_incident_status`, `enabled`)
- antifraud_incidents (`status`, `source_event_id`, `source_external_id`, `risk_level`, `rule_id`, `download_url`, `payload`)
- antifraud_incident_actions (`action_type`, `status`, `connector_id`, `result`, `error`, `started_at`, `finished_at`)
- service_alert_rules (tenant-configurable degradation alerts: `metric_type=low_rps|high_latency`, `module`, thresholds, `window_seconds`, `cooldown_seconds`, `severity`, `enabled`)
- soc_access_policies (`allowed_case_tags`, `max_cases_in_work`)
- shifts
- case_comments/case_meta
- case communication/forum entities

Anti-fraud ETL:

- для пакетной загрузки anti-fraud событий добавлен python entrypoint:
  - `./backend/tools/antifraud_etl.py`
- скрипт работает через tenant-scoped catalog API `POST/PATCH /catalog/antifraud_events`.

Anti-fraud incident runtime (tenant-scoped, analyst+):

- `GET /antifraud/incidents?page=&page_size=&q=&status=`
- `POST /antifraud/incidents/route` (`rule_id?`, `dry_run?`)
- `POST /antifraud/incidents/:incidentID/actions` (`action_type`, `connector_id?`, `recipient?`, `subject?`, `message?`)
- `PATCH /antifraud/incidents/:incidentID/status` (`status`, `resolution_summary?`, `connector_id?`, `recipient?`, `subject?`, `notify_on_close?`)

Legacy notes:

- `catalog/api_tokens` оставлен только как legacy-данные;
- deprecated kinds `analyzer_workflows`, `responder_workflows`, `action_modules`, `inbound_connectors`, `case_connectors` отвечают `410 Gone`;
- для аутентификации используется `api_access_tokens` с hash в БД и scope/full-access политикой.

## 14. Search

Tenant-scoped:

- `GET /search?q=<query>`

Search payload дополняется anti-fraud секциями:

- `antifraud_events[]` (`id`, `title`, `external_id`, `status`, `risk_level`, `download_url`)
- `antifraud_incidents[]` (`id`, `title`, `source_external_id`, `status`, `risk_level`, `download_url`)

## 15. System endpoints

- `GET /healthz`
- `GET /api/v1/system/health`
- `GET /api/v1/system/resources`
- `GET /metrics`

Dev-only:

- `GET /dev/swagger/openapi.json`
- `GET /dev/swagger`

Спецификация `openapi.json` синхронизируется с зарегистрированными роутами через `backend/tools/openapi_sync`, а drift контролируется тестом `backend/internal/api/openapi_drift_test.go`.

## 16. Асинхронные CRUD операции (Kafka)

При `ASYNC_OPS_ENABLED=true` операции:

- `POST /alerts`
- `PATCH /alerts/:alertID`
- `DELETE /alerts/:alertID`
- `POST /cases`
- `PATCH /cases/:caseID`
- `DELETE /cases/:caseID`

могут отвечать `202 Accepted` с payload:

- `operation_id`
- `resource`
- `resource_id`
- `operation_type`
- `status=queued`

Polling статуса:

- `GET /operations/:operationID` (tenant-scoped, analyst+)
  - `status`: `queued|processing|done|failed`
  - `attempt_count`, `max_attempts`
  - `error` (для `failed`)

Фоновая обработка выполняется через Kafka topic `ASYNC_OPS_KAFKA_TOPIC`.

Retry/DLQ:

- `ASYNC_OPS_RETRY_MAX_ATTEMPTS` (default `5`)
- `ASYNC_OPS_RETRY_BASE_BACKOFF` (default `500ms`)
- `ASYNC_OPS_RETRY_MAX_BACKOFF` (default `15s`)
- `ASYNC_OPS_KAFKA_DLQ_TOPIC` (default `incidenthub.async.ops.dlq`)

## 17. Авторизация и заголовки

- `Authorization: Bearer <token>` — для защищенных маршрутов
  - JWT access token (login/refresh)
  - или API access token (созданный через `/admin/api-tokens`)
- `X-Tenant-ID: <tenant-id>` — для tenant-scoped API

## 18. Коды ответов (типовые)

- `200/201/204` — успешные операции
- `202` — операция принята в async queue
- `400` — invalid params/body
- `401` — auth required / token invalid
- `403` — role forbidden
- `404` — entity not found in tenant scope
- `409` — конфликт уникальности/бизнес-constraint
- `429` — login rate limit exceeded
- `500/502/503` — внутренние и внешние integration ошибки
