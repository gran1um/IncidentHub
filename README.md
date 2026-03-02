# IncidentHub

Multi-tenant SOC platform (TheHive-style) with Go backend, React frontend, and full local infrastructure.

## Stack

- Backend: Go (`go.mod` 1.24, `toolchain go1.25.0`), Echo v5, PostgreSQL, Redis, Elasticsearch
- Frontend: React + TypeScript + Vite + pnpm
- Infra: Docker Compose (Postgres, Redis, Elasticsearch, Kafka, MinIO S3, Ollama AI model, OTel, Prometheus, Grafana)
- Security: JWT access/refresh, tenant-scoped API access tokens (scopes/full-access), RBAC, tenant isolation, secure headers, distributed login rate-limit
- Observability: anyinfra logger/tracer (local mock as in alerter), Prometheus metrics, health endpoints

## Architecture

- `frontend` calls `backend` REST API
- `backend` persists source of truth in PostgreSQL
- Elasticsearch is used for search and AI retrieval context
- Redis is used for distributed rate-limits/cache
- MinIO/AWS S3 is used for artifact storage (attachments)

### External communications (Telegram-only)

IncidentHub keeps a dedicated tenant-scoped Telegram communication layer for case discussions:

- case communication threads/messages are stored in catalog (`case_communication_thread`, `case_communication_message`)
- delivery/sync to Telegram is routed via forum proxy + outbound driver
- connector selection for communications is exposed via `GET /api/v1/communications/connectors`
- inbound/outbound connector management APIs and action-module runtime are removed from public product surface

### Forum external proxy

Forum supports proxy messaging through outbound connectors:

- send forum message to external service
- receive response back into same forum thread
- sync listener updates from external service on demand
- persist recipient aliases (username/external id -> recipient id) for Telegram-style channels

Implemented drivers:

- `mock` (dev/test)
- `webhook`
- `telegram`
- `kafka`
- `redis`

Design is pluggable for additional drivers (Outlook/email/other channels).

### AI assistant

`/api/v1/ai/ask` runs a simple security-oriented retrieval agent:

- builds tenant-scoped operational context from PostgreSQL (`cases`, `alerts`, dashboard aggregates, tenant case-status model)
- loads PostgreSQL tenant context through read-only, tenant-scoped MCP tools (allowlist + per-tool limits via `AI_MCP_*` env)
- retrieves semantic/full-text context from Elasticsearch (`alerts`, `cases`, `observables`, `forum_threads`)
- builds concise incident answer + recommendations + sources
- applies quality gate: if the model call fails (or returns empty/low-quality output), the API returns an error (no deterministic fallback answers)
- integrated into frontend AI panel
- uses per-user persistent chat sessions/messages in PostgreSQL (`ai_chat_sessions`, `ai_chat_messages`)
- supports multiple chats per user (`GET/POST /api/v1/ai/sessions`) plus backward-compatible default-session endpoint (`GET /api/v1/ai/session`)
- supports `ollama` and `openai` providers via env (`AI_PROVIDER`, `AI_ENDPOINT`, `AI_API_KEY`)
- supports case-level AI analysis history (`case_ai_analyses`) and re-run from case page
- local AI container is `ollama` with pulled model `${AI_MODEL}` (default `llama3.2:3b`)

## Repository layout

- `backend/` — API, repositories, migrations, services (forum proxy, Telegram delivery, AI)
- `frontend/` — React application (dashboard/alerts/cases/forum/templates/admin/profile)
- `deploy/` — prometheus/grafana/otel/nginx configs
- `docker-compose.yml` — local full stack
- `docs/architecture.likec4` — complete LikeC4 system diagram

LikeC4 validation/build:

- `pnpm dlx likec4 validate docs`
- `pnpm dlx likec4 build docs`

## Quick start

1. Prepare env:

```bash
cp .env.example .env
```

2. Build and run:

```bash
docker compose up -d --build
```

AI использует внешний LLM endpoint (задается в `AI_ENDPOINT`), поэтому локальный контейнер модели не запускается.

3. Open:

- Frontend: `http://localhost:5173`
- Swagger: `http://localhost:5173/swagger` (alias: `http://localhost:5173/dev/swagger`, controlled by `HTTP_EXPOSE_SWAGGER`)
- Swagger JSON: `http://localhost:5173/swagger/openapi.json` (alias: `http://localhost:5173/dev/swagger/openapi.json`)
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`

Default bootstrap admin:

- email: `admin@incidenthub.local`
- password: `ChangeMeNow123!`

Bootstrap also seeds demo data into the main `admin` tenant:

- 3 demo cases
- 5 demo alerts

Bootstrap env (first platform admin):

- `BOOTSTRAP_ENABLED=true`
- `BOOTSTRAP_PLATFORM_ADMIN_EMAIL=admin@incidenthub.local`
- `BOOTSTRAP_PLATFORM_ADMIN_USERNAME=platform-admin`
- `BOOTSTRAP_PLATFORM_ADMIN_PASSWORD=ChangeMeNow123!`
- `BOOTSTRAP_PLATFORM_ADMIN_FULL_NAME=Platform Administrator`

## Local development

### Backend

```bash
cd backend
go test ./...
go run ./cmd/api
```

Integration suite (connectors + workflow + API integration handlers):

```bash
make backend-integration-test
```

Test safety:

- integration tests must run against isolated `*_test` database
- default is `TEST_DATABASE_URL=postgres://incidenthub:incidenthub@localhost:5432/incidenthub_test?sslmode=disable`
- test harness blocks unsafe DB names (for example `incidenthub`) to prevent data loss
- if DB infra is unavailable, tests skip DB suites by default; set `INCIDENTHUB_REQUIRE_DB_TESTS=true` to fail fast instead

### Frontend

```bash
pnpm --filter incidenthub-frontend dev
```

## Main API routes

Auth:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/sessions`
- `POST /api/v1/auth/sessions/revoke-others`

Core:

- `GET /api/v1/me`
- `GET/POST /api/v1/alerts`
- `GET /api/v1/alerts/:alertID`
- `PATCH /api/v1/alerts/:alertID`
- `DELETE /api/v1/alerts/:alertID`
- `POST /api/v1/alerts/bulk/link-case`
- `POST /api/v1/alerts/bulk/create-case`
- `GET /api/v1/dashboard/stats`
- `GET /api/v1/duty/overview` (`?at=<RFC3339>&limit=<int>`, returns on-duty/current/next shifts)
- `GET/POST /api/v1/cases`
- `GET /api/v1/cases/:caseID`
- `PATCH /api/v1/cases/:caseID`
- `DELETE /api/v1/cases/:caseID`
- `GET /api/v1/case-statuses`
- `PUT /api/v1/case-statuses`
- `GET/POST /api/v1/tasks`
- `PATCH /api/v1/tasks/:taskID`
- `DELETE /api/v1/tasks/:taskID`
- `GET /api/v1/users/:id/experience-events`
- `GET /api/v1/users/:id/performance`
- `POST /api/v1/users/:id/experience-awards`
- `POST /api/v1/users/:id/media/:kind/upload` (`kind=avatar|cover`)

Case workspace:

- `GET/POST /api/v1/cases/:caseID/observables`
- `PATCH/DELETE /api/v1/cases/:caseID/observables/:observableID`
- `GET/POST /api/v1/cases/:caseID/events`
- `GET/POST /api/v1/cases/:caseID/pages`
- `GET/POST /api/v1/cases/:caseID/attachments`
- `POST /api/v1/cases/:caseID/attachments/upload`
- `GET /api/v1/cases/:caseID/attachments/:attachmentID/download`

Forum:

- `GET /api/v1/forum/threads`
- `GET /api/v1/forum/threads/:threadID`
- `POST /api/v1/forum/threads`
- `POST /api/v1/forum/posts`
- `POST /api/v1/forum/threads/:threadID/posts/upload` (multipart file/image attachments)
- `POST /api/v1/forum/threads/:threadID/proxy/send`
- `POST /api/v1/forum/threads/:threadID/proxy/sync`

Connectors:

- `GET /api/v1/communications/connectors`
- `GET/POST/PATCH/DELETE /api/v1/catalog/:kind`

AI and search:

- `GET /api/v1/ai/sessions`
- `POST /api/v1/ai/sessions`
- `GET /api/v1/ai/session`
- `GET /api/v1/ai/messages`
- `POST /api/v1/ai/ask`
- `POST /api/v1/cases/:caseID/ai/analyze`
- `GET /api/v1/cases/:caseID/ai/analyses`
- `GET /api/v1/search?q=<query>`

Workflow builder (frontend + catalog backend):

- frontend route: `/workflow-studio`
- studio surface: native IncidentHub builder
- separate Workflow AI Agent chat in studio header (prompt -> workflow JSON -> apply to canvas)
- catalog kind: `workflows` (primary)
- persisted graph shape: `definition.nodes[]`, `definition.edges[]`, `definition.entryNodeId`
- frontend node modules: `frontend/src/features/workflow-studio/nodes/<node-id>/node.ts`
- backend executable runtime: `backend/internal/workflow/` + `backend/internal/workflow/nodes/`
- built-in executable nodes:
  - `trigger`
  - `condition`, `switch`, `delay`, `scheduler`
  - `transform`, `aggregate`
  - `ioc_extract`, `watchlist_match`, `mitre_map`, `risk_score`, `containment_decision`
  - `telegram_send`
  - `postgres_query`, `redis`, `cassandra_query`, `s3_object`
  - `python_code`

### Add your own workflow node

Frontend:

1. Create `frontend/src/features/workflow-studio/nodes/<node-id>/node.ts`.
2. Export `node` manifest (`id`, `title`, `category`, `defaultConfig`, optional `fields`).
3. (Optional) Add icon/tone mapping in `frontend/src/features/workflow-studio/node-visuals.ts`.

Backend:

1. Add executor in `backend/internal/workflow/nodes/<node-id>.go` implementing `workflow.NodeExecutor`.
2. Register executor in `backend/internal/workflow/nodes/builtin.go`.
3. Keep `Type()` equal to frontend `node.id`.
4. Add tests under `backend/internal/workflow/nodes/` and `backend/internal/api/handler_workflows_test.go`.

System:

- `GET /healthz`
- `GET /api/v1/system/health`
- `GET /api/v1/system/resources`

Tenant-scoped routes require `X-Tenant-ID`.

When `ASYNC_OPS_ENABLED=true`, `POST/PATCH/DELETE` for alerts and cases may return `202 Accepted` (`status=queued`) and are processed through Kafka.

## User sessions

- Profile/Security sessions are backed by PostgreSQL `refresh_tokens` (real sessions, no mock timer)
- Session auto-expiration is controlled by `AUTH_REFRESH_TTL` (default: `24h`)
- `GET /api/v1/auth/sessions` returns:
  - active/expired/revoked state
  - per-session duration and remaining lifetime
  - `session_timeout_seconds` derived from `AUTH_REFRESH_TTL`
- `POST /api/v1/auth/sessions/revoke-others` revokes all active user sessions except the current refresh cookie session

## Extended case fields

Case create/update supports additional fields:

- `case_number`
- `source`
- `incident_type`
- `priority`
- `impact`
- `confidence (0..100)`
- `detected_at`, `occurred_at`, `closed_at` (RFC3339)
- `resolution_summary`
- `assigned_to`

## S3 artifacts

Enable via env:

- `S3_ENABLED=true`
- `S3_ENDPOINT=http://minio:9000` (or AWS S3 endpoint)
- `S3_PUBLIC_ENDPOINT=http://localhost:9000` (public host for presigned links)
- `S3_REGION=us-east-1`
- `S3_BUCKET=incidenthub-artifacts`
- `S3_ACCESS_KEY=...`
- `S3_SECRET_KEY=...`
- `S3_USE_SSL=false|true`
- `ARTIFACTS_MAX_UPLOAD_MB=50`
- `ARTIFACTS_PRESIGN_TTL=15m`

Stored in S3:

- case artifacts (`/cases/:caseID/attachments/upload`)
- profile avatar/cover uploads (`/users/:id/media/:kind/upload`)
- forum post files/images (`/forum/threads/:threadID/posts/upload`)

Important for local Docker usage:

- keep internal backend endpoint as `S3_ENDPOINT=http://minio:9000`
- expose browser-safe presigned URLs via `S3_PUBLIC_ENDPOINT=http://localhost:9000`

The frontend nginx is configured with `client_max_body_size 100m`, so uploads above 1MB are accepted (backend still enforces `ARTIFACTS_MAX_UPLOAD_MB`).

## Password hash CLI

Generate argon2id password hash from CLI:

```bash
make password-hash ARGS="-password 'StrongPassword123!'"
```

Or from stdin:

```bash
echo "StrongPassword123!" | make password-hash ARGS="-stdin"
```

## Load testing module

RPS benchmark for read/write API routes with per-step latency and error stats:

```bash
make loadtest ARGS="-base-url http://localhost:5173 -email admin@incidenthub.local -password 'ChangeMeNow123!'"
```

Alternative direct runner:

```bash
./backend/tools/loadtest/run.sh -base-url http://localhost:5173 -email admin@incidenthub.local -password "ChangeMeNow123!"
```

Report files are written to `backend/loadtest/results/` (`.json` + `.md`).

## Scaling

Backend is stateless for horizontal scale:

- session refresh tokens in PostgreSQL
- distributed login throttling in Redis
- shared API cache + invalidation versioning in Redis
- inbound deduplication in PostgreSQL (`connector_ingest_states`)
- case creation from alert batches is transactional (`create case + bind alerts` in one DB transaction)

Scale example:

```bash
docker compose up -d --scale backend=3
```

## Connector and AI env

- `CONNECTORS_INBOUND_ENABLED=true`
- `CONNECTORS_INBOUND_POLL_INTERVAL=5m`
- `CONNECTORS_INBOUND_HTTP_TIMEOUT=20s`
- `CONNECTORS_INBOUND_BATCH_LIMIT=200`
- `CONNECTORS_OUTBOUND_HTTP_TIMEOUT=15s`
- `AI_ENABLED=true`
- `AI_PROVIDER=openai-compatible|openai|ollama`
- `AI_ENDPOINT=http://host:8001/v1/chat/completions` (внешний LLM endpoint)
- `AI_API_KEY=` (рекомендуемый ключ для `openai`)
- `OPENAI_API_KEY=` (legacy alias, если уже используете это имя)
- `OPENAI_BASE_URL=https://api.openai.com/v1` (fallback endpoint для `openai`, если `AI_ENDPOINT` не задан)
- `AI_MODEL=llama3.2:3b`
- `AI_TIMEOUT=60s` (для удаленных Qwen/Llama endpoint обычно безопаснее `60s`)
- `AI_TOP_K=6`
- `AI_MAX_CONTEXT_CHARS=5000`
- `AI_HISTORY_MESSAGES=20`
- `AI_MCP_ENABLED=true`
- `AI_MCP_TOOLS_ALLOWLIST=open_case_statuses,cases_in_work,recent_cases,recent_alerts,dashboard_stats`
- `AI_MCP_PER_TOOL_LIMIT=5`
- `AI_SYSTEM_PROMPT=...`
- `ASYNC_OPS_ENABLED=false`
- `ASYNC_OPS_KAFKA_BROKERS=kafka:9092`
- `ASYNC_OPS_KAFKA_TOPIC=incidenthub.async.ops`
- `ASYNC_OPS_KAFKA_GROUP_ID=incidenthub-api-async-ops`
- `ASYNC_OPS_PRODUCE_TIMEOUT=5s`
- `ASYNC_OPS_POLL_TIMEOUT=1s`
- `SERVICE_ALERTING_ENABLED=true`
- `SERVICE_ALERTING_EVALUATION_INTERVAL=30s`
- `SERVICE_ALERTING_DEFAULT_WINDOW=5m`
- `SERVICE_ALERTING_DEFAULT_COOLDOWN=15m`
- `SERVICE_ALERTING_RULES_LIMIT=500`
- `SERVICE_ALERTING_RECIPIENTS_LIMIT=300`

When `ASYNC_OPS_ENABLED=true`, alert/case create+update+delete requests are queued to Kafka and API returns `202 Accepted` with `operation_id` + target `resource_id`.

Service degradation rules are configured tenant-side in `catalog/service_alert_rules` (metric types: `low_rps`, `high_latency`) and deliver alerts through the existing notification Kafka -> Telegram pipeline.

Kafka/Telegram/S3 connector endpoints, auth and transport parameters are configured in each connector payload (`data.config`) and are not taken from global connector env defaults.

## Redis API cache env

- `API_CACHE_ENABLED=true`
- `API_CACHE_TTL=30s`
- `API_CACHE_MAX_BODY_BYTES=1048576`

## Security model

Roles:

- `platform_admin`
- `tenant_admin`
- `analyst`
- `viewer`

User registration policy:

- only `platform_admin` or `tenant_admin` can create users

LDAP:

- LDAP auth scaffold is present (`backend/internal/auth/ldap.go`)
- enabled by env (`LDAP_ENABLED`, `LDAP_URL`, ...)

## Tests

Backend tests and static checks:

```bash
cd backend
GOCACHE=/tmp/incidenthub-gocache go test ./...
go vet ./...
# coverage (exclude tooling commands)
GOCACHE=/tmp/incidenthub-gocache go list ./... \
  | grep -v '/tools/loadtest$' \
  | grep -v '/tools/openapi_sync$' \
  | xargs go test -count=1 -coverprofile=coverage.out
GOCACHE=/tmp/incidenthub-gocache go tool cover -func=coverage.out | tail -n 1

# sync OpenAPI spec with registered routes
GOCACHE=/tmp/incidenthub-gocache go run ./tools/openapi_sync
```

Backend integration suite (requires Docker services: Postgres, Redis, Kafka, MinIO):

```bash
make backend-integration-test
```

Frontend unit/component tests (Vitest + RTL):

```bash
pnpm --filter incidenthub-frontend test:unit
pnpm --filter incidenthub-frontend test:coverage
```

Frontend dead-code hygiene check:

```bash
cd frontend
pnpm exec tsc --noEmit --noUnusedLocals --noUnusedParameters
```

Frontend/API smoke:

```bash
./frontend/tests/smoke.sh
```

If smoke fails with missing default catalog items (`observable_types`/`achievements`), restore them with forward migration `backend/migrations/000012_repair_catalog_defaults.sql` and rerun smoke.

Smoke validates:

- auth + tenant context
- alerts/cases/forum CRUD
- one-thread-per-case constraint
- outbound forum proxy send
- communication connector list endpoint
- AI ask endpoint
- system health/resources
- default catalog seeds
- search behavior
- swagger JSON route
