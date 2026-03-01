# Тестирование и контроль качества

## 1. Backend тесты

Покрываются unit и integration-like сценарии:

- AI provider/service;
- LDAP authentication flow (`backend/internal/auth/ldap_test.go`);
- API handlers (auth/sessions, alerts/cases, case statuses, AI, forum, case communications);
- cross-tenant single-entity case collaboration (`/cases/:caseID/share`, `/cases/:caseID/shares`);
- async CRUD queue для alerts/cases (Kafka-backed path);
- AI Agent queue dispatch для автоматически созданных alerts/cases (`ai_agent_queue_events`);
- stage-based AI agent investigations (`investigation_plan`) с enrichment connectors;
- auto-close кейса AI-агентом по verdict + alert->case auto-create path;
- list search modes: `plain`, `regex`, `fulltext` (PostgreSQL FTS with websearch syntax);
- anti-fraud incident runtime (rules routing, incident actions, close-email automation, search download links);
- DataSec incident runtime (register/ping/escalate/measure/close + metrics/FAQ + required integration fields);
- case copy endpoint (`POST /cases/:caseID/copy`);
- optimistic concurrency for case patch (`expected_updated_at` -> `409` on stale update);
- workflow runtime API (`run/test-run/list-runs`) и built-in node executors;
- безопасная смена пароля (проверка `current_password`);
- XP rewards/events/performance endpoints;
- outbound connectors and durable Connector Hub runtime;
- catalog ACL для metadata-сущностей (`case_meta`, `alert_meta`);
- middleware/cache/metrics/security;
- repositories/database/storage/search.

Ключевые файлы:

- `./backend/internal/api/handler_alerts_cases_async_test.go`
- `./backend/internal/api/handler_ai_agents_queue_integration_test.go`
- `./backend/internal/api/async_ops_kafka_integration_test.go`
- `./backend/internal/api/handler_experience_rewards_test.go`
- `./backend/internal/api/handler_workflows_test.go`
- `./backend/internal/api/handler_notifications_test.go`
- `./backend/internal/api/handler_antifraud_incidents_integration_test.go`
- `./backend/internal/api/handler_datasec_incidents_integration_test.go`
- `./backend/internal/api/handler_tenants_users_branches_test.go`
- `./backend/internal/api/handler_workflows_integration_test.go`
- `./backend/internal/connectors/outbound/service_integration_test.go`
- `./backend/internal/workflow/nodes/new_nodes_test.go`
- `./backend/internal/security/redis_rate_limiter_test.go`
- `./backend/internal/repository/*_test.go`
- `./backend/internal/repository/ai_agent_queue_repository_integration_test.go`

Основные команды:

- `make backend-test`
- `make backend-integration-test`
- `cd backend && go vet ./...`
- `cd backend && go list ./... | grep -Ev '/tools/(loadtest|openapi_sync)$' | xargs go test -count=1 -coverprofile=coverage.out`

## 2. Frontend тестирование

Есть два контура:

1) Unit/component (Vitest + React Testing Library)

- `./frontend/tests/unit/lib/i18n.test.tsx`
- `./frontend/tests/unit/pages/operations/alerts-page.test.tsx`
- `./frontend/tests/unit/pages/operations/cases-page.test.tsx`
- `./frontend/tests/unit/pages/admin/administration-page.test.tsx`
- `./frontend/tests/unit/pages/admin/security-page.test.tsx`
- `./frontend/tests/unit/pages/operations/dashboard-page.test.tsx`
- `./frontend/tests/unit/pages/admin/workflow-studio-page.test.tsx`
- `./frontend/tests/unit/lib/workflow-node-library.test.ts`
- `./frontend/tests/unit/pages/operations/case-detail-page.test.tsx`
- `./frontend/tests/unit/pages/operations/case-communications-tab.test.tsx`

Покрываемые сценарии:

- regression по белому экрану вкладки Administration;
- generic observable-to-connector routing and connector method selection for outbound Connector Hub;
- удаление alerts/cases (single + bulk);
- fulltext search mode for cases/alerts (`search_mode=fulltext`, `search_logic=all|any`, URL-state restore);
- массовые операции alerts/cases: close/status/tag;
- case workspace: создание `Case Page` из вкладки `Pages`;
- case creation: блокировка `Create Case` без обязательных интеграционных полей (`mdm`, `scenario_id`, `incident_date`);
- case detail: copy action from header + navigation to created case;
- case detail `Visuals`: реактивная фильтрация через applet-строки (source/type) и reset linked filters;
- case communications: send с `template_id/template_vars` и sync thread по `subject`;
- cases list: скрытие закрытых по умолчанию + загрузка закрытых по кнопке;
- administration connectors: outbound connector configuration and durable execution controls;
- dashboard metrics: overdue/SLA/custom metric snapshots и правила подсветки проблемных кейсов;
- workflow studio: список/редактор/валидация block library;
- DataSec incidents page: регистрация инцидента, ping action, метрики и FAQ;
- dashboard duty overview: отображение current/next duty из `/api/v1/duty/overview`;
- базовые i18n fallback/resolution пути.

Команды:

- `pnpm --filter incidenthub-frontend test:unit`
- `pnpm --filter incidenthub-frontend test:coverage`

2) Smoke e2e

- `./frontend/tests/smoke.sh`
- smoke дополнен проверкой auto-dispatch AI Agent queue (создание AI агента + кейса и ожидание `ai_agent_runs`).

Команда:

- `./frontend/tests/smoke.sh`
- strict-режим для queue-check: `AI_QUEUE_SMOKE_STRICT=true ./frontend/tests/smoke.sh`

## 3. Build/validation команды

- `pnpm --filter incidenthub-frontend build`
- `cd frontend && pnpm exec tsc --noEmit --noUnusedLocals --noUnusedParameters`
- `pnpm docs:likec4:validate`
- `pnpm docs:likec4:build`

## 4. Coverage (актуальный прогон: 2026-02-18)

Backend (`backend/coverage.out`, `go tool cover -func`):

- `63.4% statements` (среднее по backend, полный прогон с исключением tooling-пакетов).

Frontend (`pnpm --filter incidenthub-frontend test:coverage`, include: `i18n.ts`, `alerts.tsx`, `cases.tsx`, `administration.tsx`):

- `65.48% statements/lines`;
- `51.90% branches`;
- `11.96% functions` (низко из-за большого числа inline UI callbacks в page-компонентах).

Критичные зоны 80%+:

- backend `internal/security` — `86.2%`;
- backend `internal/settings` — `88.2%`;
- backend `JWTAuth` middleware function coverage — `87.5–100%` по ключевым auth-веткам;
- frontend `alerts.tsx` — `81.66%`;
- frontend `cases.tsx` — `81.77%`;
- frontend `src/lib/i18n.ts` — `99.72%`.

## 5. Актуальный статус (последний прогон: 2026-02-20)

- backend tests: green (`make backend-test`)
- `go vet`: green
- frontend unit: green
- frontend no-unused check (`tsc --noUnusedLocals --noUnusedParameters`): green
- frontend coverage: green (thresholds соблюдены)
- frontend smoke: failed in текущем стенде (`default observable types missing, count=0`)
- frontend build: green
- LikeC4 validate/build: green (перепроверено 2026-02-19)

Для smoke требуется наличие глобальных seed-справочников (`observable_types`, `achievements`).
Если они были удалены из dev БД, восстановите их forward-миграцией `backend/migrations/000012_repair_catalog_defaults.sql` и повторите `./frontend/tests/smoke.sh`.

## 6. Ограничения и следующий фокус

- низко покрытые backend-пакеты: `cmd/api`, `internal/cache`, `internal/storage`, `internal/middleware`;
- для frontend следующая цель — поднять branch/function coverage по `administration.tsx` через декомпозицию и отдельные component tests.
