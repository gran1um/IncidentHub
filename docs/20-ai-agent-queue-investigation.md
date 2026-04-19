# AI Agent Queue + автономное расследование кейсов/алертов

Дата актуализации: 2026-03-06

## 1. Что реализовано

Сейчас backend работает по workload-модели и execution-policy-модели одновременно:

1. создание `case`/`alert` ->
2. enqueue в `ai_agent_queue_events` ->
3. воркер берет queue-event ->
4. матчит подходящих агентов ->
5. строит execution plan с учетом policy и priority ->
6. на каждого выбранного агента создает/обновляет отдельный workload в `ai_agent_workloads` ->
7. исполняет только допустимые workloads, не перезапуская уже успешные ->
8. сохраняет run в `catalog(kind=ai_agent_runs)` ->
9. синхронизирует итоговый статус queue-event из фактического состояния workloads ->
10. отдает trace по сущности в case-detail / alert-detail и в monitoring.

Это закрывает главный риск прошлой реализации: успешный агент больше не запускается повторно только потому, что другой агент внутри того же queue-event ушел в retry.

## 2. Где в коде

Ключевые изменения:

- очередь событий:
  - `./backend/migrations/000026_ai_agent_queue_events.sql`
  - `./backend/internal/models/ai_agent_queue_event.go`
  - `./backend/internal/repository/ai_agent_queue_repository.go`
- per-agent workloads + execution policy fields:
  - `./backend/migrations/000028_ai_agent_workloads.sql`
  - `./backend/migrations/000029_ai_agent_workload_execution_policy.sql`
  - `./backend/internal/models/ai_agent_workload.go`
  - `./backend/internal/repository/ai_agent_workload_repository.go`
- queue worker / dispatch / dedupe / fallback orchestration:
  - `./backend/internal/api/handler_ai_agents_queue.go`
- agent runtime / definition parsing / manual run / policy gates:
  - `./backend/internal/api/handler_ai_agents.go`
- monitoring / entity trace / restart / close:
  - `./backend/internal/api/handler_ai_agents_monitor.go`
- routing / wiring:
  - `./backend/internal/api/handler.go`
  - `./backend/internal/api/server.go`
  - `./backend/cmd/api/main.go`
- frontend monitoring + builder UX + entity trace UI:
  - `./frontend/src/lib/api/ai.ts`
  - `./frontend/src/components/ai-entity-trace.tsx`
  - `./frontend/src/pages/ai-agents.tsx`
  - `./frontend/src/pages/case-detail.tsx`
  - `./frontend/src/pages/alert-detail.tsx`

## 3. Модель данных

### 3.1 Queue events

`ai_agent_queue_events` хранит ingress-событие на сущность `(tenant_id, entity_type, entity_id)`.

Поля:

- `entity_type`: `case | alert`
- `status`: `queued | processing | done | failed`
- `matched_agents`, `processed_agents`
- `attempt_count`, `max_attempts`
- `workflow_id`, `last_error`
- `closed_by_user`, `closed_at`
- `started_at`, `finished_at`, `updated_at`

Уникальный индекс на `(tenant_id, entity_type, entity_id)` сохраняет идемпотентность enqueue.

### 3.2 Workloads

`ai_agent_workloads` хранит отдельное выполнение конкретного агента внутри queue-event.

Поля:

- `queue_event_id`
- `agent_id`, `agent_name`
- `entity_type`, `entity_id`
- `status`: `queued | processing | done | failed | cancelled`
- `attempt_count`, `max_attempts`
- `workflow_id`, `run_id`
- `last_stage`, `last_error`
- `execution_policy`: `all_matching | exclusive | first_match | fallback_chain`
- `execution_priority`: целое число для сортировки кандидатов
- `execution_index`: порядок внутри fallback chain
- `closed_by_user`, `closed_at`
- `started_at`, `finished_at`, `updated_at`

Уникальность: `(queue_event_id, agent_id)`.

Именно workload теперь является единицей retry/restart/close.

## 4. Execution policy и dispatch

Перед созданием workloads воркер строит execution plan из matched agents.

Порядок сортировки кандидатов:

1. по `execution_priority` (по убыванию);
2. по имени агента;
3. по `agent_id`.

Поддерживаемые policy:

- `all_matching` — запускаются все matched agents;
- `exclusive` — если есть хотя бы один exclusive-агент, исполняется только лучший из них;
- `first_match` — если exclusive нет, но есть first-match, исполняется только лучший из них;
- `fallback_chain` — если нет exclusive/first-match, matched fallback-агенты строятся в цепочку, и каждый следующий стартует только если предыдущий не завершился успехом.

Дополнительно для `fallback_chain`:

- workload с более поздним `execution_index` ждет завершения предыдущего;
- если предыдущий fallback workload завершился `done`, все следующие автоматически переводятся в `cancelled`;
- queue-event считается `done`, если хотя бы один fallback workload завершился успехом.

## 5. Как исполняется queue-event

Для каждого queue-event worker:

1. грузит сущность (`case` или `alert`);
2. находит matching agents;
3. строит execution plan с учетом policy;
4. делает upsert workloads только для выбранного execution plan;
5. берет только workloads в статусе `queued`;
6. для каждого workload решает действие:
   - `execute` — можно исполнять сейчас;
   - `wait` — надо дождаться предыдущего fallback workload;
   - `cancel` — цепочка уже закрыта более ранним успешным workload;
7. переводит workload в `processing`;
8. выполняет агента;
9. по результату переводит workload в:
   - `done` — успех,
   - `queued` — retry нужен,
   - `failed` — лимит попыток исчерпан,
   - `cancelled` — workload закрыт вручную или автоматически как лишний fallback.

После этого queue-event синхронизируется из workloads:

- есть `processing` -> event остается `processing`;
- при `fallback_chain`:
  - есть хотя бы один `done` -> event `done`;
  - иначе есть `queued` -> event `queued`;
  - иначе есть `failed` -> event `failed`;
- для остальных policy:
  - есть `queued` -> event `queued`;
  - есть `failed` -> event `failed`;
  - все завершены (`done/cancelled`) -> event `done`.

## 6. Trace по сущности и UI

Добавлен entity-level trace endpoint:

- `GET /api/v1/ai/agents/entities/:entityType/:entityID`

Параметры:

- `entityType`: `case|cases|alert|alerts`
- `entityID`: UUID сущности
- `limit`: `1..20`, default `8`

Response:

- `entity_type`
- `entity_id`
- `events[]`
- `active_event_id`
- `total`

Каждый `events[]` snapshot включает:

- статус queue-event;
- `execution_policy` события;
- candidate agents;
- вложенные `workloads[]`;
- snapshot `run` и `run.result`, если запуск уже состоялся.

Каждый workload snapshot включает:

- `execution_policy`, `execution_priority`, `execution_index`;
- `attempt_count/max_attempts`;
- `last_stage`, `last_error`;
- `run.result.summary`, `run.result.verdict`, `run.result.action_blockers`;
- `run.result.stage_timeline[]` — first-class этапы расследования;
- `run.result.connector_timeline[]` — first-class вызовы коннекторов с status/duration/reply/error.

На frontend это встроено так:

- в `./frontend/src/pages/case-detail.tsx` появился отдельный таб `AI Workloads`;
- в `./frontend/src/pages/alert-detail.tsx` появился AI trace card;
- в `./frontend/src/components/ai-entity-trace.tsx` общий секционный UI для trace без глобального skeleton на весь экран;
- прямо в case/alert trace доступны inline workload controls `Restart` / `Close` для tenant admin / platform admin;
- на case trace добавлены in-context human intervention actions `Assign to me` и `Escalate`, чтобы аналитик не уходил со страницы кейса;
- на alert trace добавлены inline actions `Assign to me` и `Open Linked Case` / `Create Case`, чтобы human handoff делался прямо из AI блока;
- trace теперь показывает stage timeline и connector timeline по каждому workload, а не только итоговый summary/error;
- query params `ai_workload` и `ai_stage` подсвечивают конкретный workload/stage inside entity trace и прокручивают страницу к нужному месту;
- на `AI Agents -> Operations` появился side drawer trace-inspector, который открывается прямо из summary card и не уводит пользователя со страницы мониторинга.

## 7. Alert -> Case дедупликация

Для `alert` flow при `auto_create_case_from_alert=true` агент перед созданием кейса всегда перечитывает свежий `alert` из БД.

Это важно для multi-agent dispatch:

- первый агент может создать кейс и привязать `alert.case_id`;
- второй агент при старте уже увидит обновленный alert и возьмет существующий кейс;
- duplicate case creation не происходит.

## 8. Stage-plan и enrichment

Агент поддерживает creator-defined `investigation_plan`.

Каждый stage может включать:

- `name`
- `description`
- `prompt`
- `enrichment_connector_ids`
- `case_tags`

Исполнение:

1. stage prompt попадает в контекст LLM;
2. enrichment connectors вызываются на нужном этапе;
3. stage tags и `auto_case_tags` применяются к `case_meta`;
4. итог stage execution попадает в run result.

Если `investigation_plan` пустой, используется fallback-путь на основе `prompt` и глобальных enrichment connectors агента.

## 9. Policy-гейты авто-действий

Поддерживаются:

- `auto_close_case`
- `auto_close_verdicts`
- `notification_connector_ids`
- `require_reviewer_consensus`
- `auto_action_min_confidence`
- `triad_enabled`

Важно: `auto_action_min_confidence` теперь работает даже без triad.

Авто-действия блокируются, если:

- `confidence < auto_action_min_confidence`;
- triad требует reviewer consensus, но consensus нет;
- `requires_human_review=true`;
- есть triad warnings/errors.

При блокировке в result пишутся:

- `auto_actions_allowed=false`
- `action_blockers`
- `auto_close_skipped_reason`
- `notifications_skipped_reason`

## 10. Manual Run и monitoring

### 10.1 Manual Run

`POST /api/v1/ai/agents/{agentID}/run` поддерживает:

- `case_ids`
- `alert_ids`
- `max_cases`
- `dry_run`
- `language`

Для `alert_ids` backend сам резолвит alert в case через тот же flow, что и queue worker.

В response и persisted run сохраняются:

- `requested_case_ids`
- `requested_alert_ids`

### 10.2 Operations overview

`GET /api/v1/ai/agents/ops/overview` возвращает не только queue-events, но и вложенные workloads:

- `processing_events[*].workloads`
- `recent_failed_events[*].workloads`
- `workload_count`
- `execution_policy`
- snapshot run/result для workload, если он уже есть
- внутри `run.result` доступны `stage_timeline` и `connector_timeline`, поэтому monitoring может показывать не только verdict, но и ход расследования по этапам.

На frontend это теперь используется напрямую в `AI Agents -> Operations`:

- expanded workload panel рендерит тот же `AIEntityTrace`, что и case/alert pages;
- SOC-оператор видит stage timeline, connector timeline, blockers и ошибки без реконструкции из runs feed;
- restart/close workload можно делать прямо из Operations, не переходя в отдельную сущность;
- для каждого workload есть deep link в сам case/alert и отдельный deep link в конкретный stage внутри trace (`ai_workload` + `ai_stage`);
- из summary cards можно открыть side drawer с живым trace, а из triad analytics — сразу провалиться в case AI tab.

### 10.3 Точечные controls

Добавлены workload-level endpoints:

- `POST /api/v1/ai/agents/ops/workloads/{workloadID}/restart`
- `POST /api/v1/ai/agents/ops/workloads/{workloadID}/close`

Event-level controls оставлены:

- `POST /api/v1/ai/agents/ops/events/{eventID}/restart`
- `POST /api/v1/ai/agents/ops/events/{eventID}/close`

Но точный restart/close рекомендуется делать на уровне workload.

### 10.4 Builder UX

На экране AI Agents доработано:

- builder сохраняет `executionPolicy` и `executionPriority`;
- monitoring использует реальные `workloads`, а не только эвристику по runs feed;
- restart/close дергают workload endpoints;
- у stage-plan сохраняется `description`;
- `Task Assignee` выбирается из tenant users, а не через raw UUID;
- manual run позволяет передавать `case_ids` и `alert_ids`.

## 11. Runtime env

Используются:

- `AI_AGENT_QUEUE_ENABLED=true`
- `AI_AGENT_QUEUE_POLL_INTERVAL=3s`
- `AI_AGENT_QUEUE_BATCH_SIZE=20`
- `AI_AGENT_QUEUE_RUN_TIMEOUT=90s`
- лимит параллельных LLM-запросов задается общим AI concurrency limiter в сервисе AI

## 12. Тесты

### 12.1 Backend integration

Покрыто в:

- `./backend/internal/api/handler_ai_agents_queue_integration_test.go`
- `./backend/internal/api/handler_ai_agents_integration_test.go`
- `./backend/internal/repository/ai_agent_queue_repository_integration_test.go`

Проверяется:

- enqueue / re-enqueue queue-event;
- multi-agent workload dispatch;
- execution policy `exclusive`, `first_match`, `fallback_chain`;
- отсутствие повторного запуска успешного workload при retry другого workload;
- single-case creation для `alert` при нескольких matching agents;
- confidence gate без triad;
- manual run по `alert_ids`;
- workload-level restart/close;
- entity trace endpoint;
- stale recovery и queue transitions.

Команда проверки:

```bash
go test ./internal/api/... ./internal/repository/... ./internal/models/...
```

### 12.2 Frontend

Unit-тесты обновлены для:

- builder policy fields;
- operations diagnostics;
- workload restart/close controls;
- case-detail AI tab;
- alert-detail AI trace card.

Команда проверки:

```bash
pnpm --filter ./frontend exec vitest run tests/unit/pages/admin/ai-agents-page.test.tsx tests/unit/pages/operations/case-detail-page.test.tsx tests/unit/pages/operations/alert-detail-page.test.tsx
```

## 13. Что еще остается

Следующие эволюционные шаги:

1. вынести side effects в отдельную сущность `ai_agent_actions`, чтобы у каждого workload был полноценный action log;
2. сделать stage timeline и connector timeline first-class объектами мониторинга, а не только частью `run.result`;
3. добавить in-context controls в case/alert trace: restart/escalate/reassign без перехода в admin monitoring;
4. после возврата Temporal использовать `ai_agent_queue_events` как ingress, а `ai_agent_workloads` как runtime state per-agent;
5. добавить policy уровня rollout (`draft/published/canary`) и capability-aware connector validation перед включением агента.
