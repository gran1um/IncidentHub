# PostgreSQL: схема, индексы, ограничения

## 1. Общая роль PostgreSQL

PostgreSQL — source of truth платформы:

- identity/membership и refresh-сессии;
- alerts/cases/tasks и case workspace;
- connector runtime state;
- workflow runtime history;
- notification bot/settings state;
- forum/case communication bindings;
- AI sessions/history/analysis;
- опыт пользователей и правила наград (XP).

Версия в инфраструктуре: `postgres:18`, заявлена совместимость с 17/18.

## 2. Миграционная модель

Актуальная цепочка миграций:

- `000001_init.sql` ... `000011_case_communications_indexes.sql`
- `000012_repair_catalog_defaults.sql`
- `000013_user_experience_rewards.sql`
- `000014_user_xp_event_descriptions.sql`
- `000015_connector_recipient_aliases.sql`
- `000016_api_access_tokens.sql`
- `000017_async_operations.sql`
- `000018_workflow_runs.sql`
- `000019_telegram_notification_delivery.sql`
- `000020_ai_chat_session_ordering.sql`
- `000021_achievement_icon_pack.sql`
- `000026_ai_agent_queue_events.sql`

Ключевые свойства:

- `schema_migrations` создается автоматически;
- миграции применяются по имени файла;
- каждый SQL-файл выполняется в отдельной транзакции;
- применение защищено `pg_advisory_lock` в `ApplyMigrations`, чтобы избежать гонок при конкурентном старте нескольких инстансов.

## 3. Основные таблицы

### 3.1 Identity и доступ

- `tenants`
- `users`
- `tenant_memberships`
- `refresh_tokens`

Ключевые ограничения:

- `tenants.slug` — `UNIQUE`
- `users.username`, `users.email` — `UNIQUE`
- `tenant_memberships` — `UNIQUE(tenant_id, user_id)`
- membership role check: `tenant_admin|analyst|viewer`

Поле профиля:

- `users.experience_points BIGINT NOT NULL DEFAULT 0`

### 3.2 SOC core

- `alerts`
- `cases`
- `tasks`
- `audit_logs`

Расширения `cases`:

- `case_number`, `source`, `incident_type`, `priority`, `impact`, `confidence`, `detected_at`, `occurred_at`, `closed_at`, `resolution_summary`
- check: `confidence between 0..100`
- `uq_cases_tenant_case_number` — уникальный номер кейса в тенанте

### 3.3 Case workspace

- `case_observables`
- `case_timeline_events`
- `case_pages`
- `case_attachments`

Ограничения:

- `UNIQUE (tenant_id, case_id, type, value)` для observables.

### 3.4 Универсальный каталог

- `catalog_items` (JSONB-ядро для расширяемых сущностей)

Через `kind` реализуются:

- templates/case_templates
- observable_types
- achievements
- case_comments, case_meta
- connectors/methods/automations
- shifts
- case_statuses
- case_communication_thread/case_communication_message
- forum_thread/forum_post

### 3.5 Connectors runtime

- `connector_ingest_states`
- `inbound_connector_runs`
- `forum_external_bindings`
- `connector_recipient_aliases`
- `api_access_tokens`
- `async_operations`

Ограничения:

- dedup: `UNIQUE (tenant_id, connector_id, external_id)`
- inbound run status check: `running|success|error|skipped`
- inbound run trigger check: `cron|manual`
- forum external binding: `UNIQUE (tenant_id, thread_id, connector_id)`
- recipient alias: `UNIQUE (tenant_id, connector_id, channel, alias_type, alias_value)`
- api token: `token_hash UNIQUE`, active-токены фильтруются по `revoked_at/expires_at`
- async operation status: `status IN ('queued','processing','done','failed')`

### 3.6 Workflow runtime

- `workflow_runs`

Ограничения:

- `workflow_runs.status IN ('queued','running','success','failed')`
- FK `workflow_id -> catalog_items(id)` с `ON DELETE CASCADE`
- retention в API-слое: хранится до 1000 последних запусков на workflow (`DELETE old rows` после записи нового запуска).

### 3.7 Notification delivery settings

- `telegram_notification_bots`
- `user_notification_settings`

Ограничения:

- bot identity uniqueness: `UNIQUE (tenant_id, bot_id)`, `UNIQUE (tenant_id, bot_username)`
- user settings uniqueness: `UNIQUE (tenant_id, user_id)`
- `user_notification_settings.telegram_bot_id -> telegram_notification_bots(id)` с `ON DELETE SET NULL`

### 3.8 AI persistence

- `ai_chat_sessions`
- `ai_chat_messages`
- `case_ai_analyses`

Ограничения:

- один default чат на `tenant+user` (`uq_ai_chat_sessions_default`)
- роли сообщений: `system|user|assistant`
- case AI status: `completed|failed`

### 3.9 XP и reward rules

- `xp_reward_rules`
  - `key TEXT PRIMARY KEY`
  - `points INTEGER CHECK (points >= 0)`
  - `description`, `updated_at`
- `user_xp_events`
  - `id UUID PRIMARY KEY`
  - `tenant_id` (nullable FK)
  - `user_id` (FK -> users)
  - `event_key TEXT UNIQUE` (идемпотентность начислений)
  - `event_type`, `points`, `description`, `details JSONB`, `created_at`

`000014` добавляет `user_xp_events.description` и backfill для исторических записей.

### 3.10 AI agent runtime queue

- `ai_agent_queue_events`

Назначение:

- очередь автоматического запуска AI-агентов для новых `case`/`alert` сущностей;
- хранение статуса обработки (`queued|processing|done|failed`) и агрегатов (`matched_agents`, `processed_agents`).

Ограничения:

- `entity_type IN ('case','alert')`
- `status IN ('queued','processing','done','failed')`
- `UNIQUE (tenant_id, entity_type, entity_id)` для дедупликации queue events.

## 4. Индексы

### 4.1 Tenant-centric индексы

Примеры:

- `idx_alerts_tenant_updated`
- `idx_cases_tenant_updated`
- `idx_tasks_tenant_status`
- `idx_case_observables_case_updated`

### 4.2 Фильтрация по статусам/серьезности

- `idx_alerts_tenant_status_severity`
- `idx_cases_tenant_status_severity`

### 4.3 Full-text в PostgreSQL

- `idx_alerts_text_search` (GIN to_tsvector)
- `idx_cases_text_search` (GIN to_tsvector)

### 4.4 JSONB/catalog

- `idx_catalog_items_data_gin`

### 4.5 Runtime/форум/notifications/workflow

- `idx_connector_ingest_states_tenant_connector`
- `idx_inbound_connector_runs_tenant_connector_started`
- `idx_forum_external_bindings_tenant_thread`
- `idx_catalog_items_case_communication_threads`
- `idx_catalog_items_case_communication_messages`
- `idx_catalog_items_case_communication_messages_external_id`
- `idx_connector_recipient_aliases_lookup`
- `idx_connector_recipient_aliases_recipient`
- `idx_api_access_tokens_tenant_created`
- `idx_api_access_tokens_tenant_active`
- `idx_api_access_tokens_hash_active`
- `idx_async_operations_tenant_requested`
- `idx_async_operations_status`
- `idx_workflow_runs_tenant_workflow_started`
- `idx_telegram_notification_bots_tenant_enabled`
- `idx_user_notification_settings_tenant_user`
- `idx_ai_agent_queue_events_status_created`
- `idx_ai_agent_queue_events_tenant_status`

### 4.6 XP

- `idx_user_xp_events_user_created` (`user_id, created_at DESC`)

## 5. Критичные constraint-правила

- 1 кейс -> 1 форум thread: `uq_catalog_forum_thread_case` (partial unique index)
- уникальность tenant slug;
- уникальность user email/username;
- уникальность `user_xp_events.event_key` для защиты от двойных начислений;
- enum-подобные проверки через `CHECK`.

## 6. Транзакционность

В транзакциях выполняются:

- миграции схемы;
- создание кейса + массовая привязка алертов;
- начисление XP (вставка event + инкремент `users.experience_points`).

## 7. Эксплуатационные заметки

- `000012_repair_catalog_defaults.sql` — repair-forward миграция для восстановления глобальных `observable_types` и `achievements` без правки старых миграций;
- `000021_achievement_icon_pack.sql` — обновляет глобальные достижения на минималистичные icon-пути (`/achievement-icons/*.svg`) и добавляет новые записи `Playbook Automator` / `Telemetry Guardian`;
- `catalog_items`, `workflow_runs`, `audit_logs`, `ai_chat_messages`, `user_xp_events` требуют регулярного контроля роста и индексов;
- attachments хранят только metadata в PostgreSQL, payload — в S3.
