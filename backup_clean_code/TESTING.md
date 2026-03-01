# IncidentHub Testing Guide

This doc describes the recommended test pyramid for IncidentHub: unit -> integration -> full-stack smoke -> manual QA.

## Quick gate (before pushing)

From repo root:

```bash
make backend-test
make backend-integration-test
(cd frontend && pnpm test:unit)
./frontend/tests/smoke.sh
bash frontend/tests/communications-sync-smoke.sh
```

Optional but recommended:

```bash
(cd backend && $(go env GOPATH)/bin/golangci-lint run ./...)
(cd frontend && pnpm lint)
(cd frontend && pnpm build)
```

## Backend (Go)

### Unit tests (fast)

```bash
cd backend
go test ./...
```

Or via Makefile:

```bash
make backend-test
```

Success criteria: exit 0, no FAILED packages.

### Integration tests (DB / external services)

Prereq: docker services running (Postgres/Redis/Kafka/MinIO).

```bash
docker compose up -d postgres redis kafka minio
```

Run integration tests:

```bash
make backend-integration-test
```

Success criteria: exit 0.

### Race detection (optional)

```bash
cd backend
INCIDENTHUB_REQUIRE_DB_TESTS=true go test -race ./...
```

Goal: catch data races in workers/queues/hub.

### Static analysis (required in CI)

Install and run golangci-lint (v2.4.0+):

```bash
cd backend
mkdir -p .cache/golangci-lint .cache/go-build
GOLANGCI_LINT_CACHE=$(pwd)/.cache/golangci-lint \
GOCACHE=$(pwd)/.cache/go-build \
$(go env GOPATH)/bin/golangci-lint run ./...
```

Success criteria: 0 issues.

## Frontend (Vite/React/TS)

### Unit tests

```bash
cd frontend
pnpm test:unit
```

Success criteria: 0 failed suites.

### Typecheck + build

```bash
cd frontend
pnpm build
```

Success criteria: TS has no errors; Vite build succeeds.

### ESLint

```bash
cd frontend
pnpm lint
```

Notes:
- `frontend/coverage/` is ignored.
- Lint is run with `--max-warnings 0` (warnings must be 0).

## Full-stack smoke (API-level e2e)

These scripts test the system as a whole: backend + DB + queues + frontend container (if enabled) + critical flows.

### General smoke

```bash
./frontend/tests/smoke.sh
```

Covers (high-level):
- login/token/tenant bootstrap
- create case + alert
- forum thread + posts
- create outbound connectors + proxy send
- basic connector hub endpoints
- queue/AI smoke (if enabled)

### Communications sync smoke (Slack/Outlook)

```bash
bash frontend/tests/communications-sync-smoke.sh
```

Success criteria: exit 0 and JSON shows `communication_created_count >= 1` and `forum_created_count >= 1`.

## Browser-level E2E (Playwright)

These tests validate real UI flows in a real browser.

Prereq:
- `docker compose up -d backend frontend` (or the full stack)
- frontend deps installed (`cd frontend && pnpm install`)

One-time (if Playwright browsers are missing):

```bash
cd frontend
pnpm exec playwright install
```

Run:

```bash
cd frontend
pnpm test:e2e
```

Notes:
- Tests live in `frontend/tests/e2e/` and use `data-testid` selectors.
- Base URL defaults to `http://localhost:5173` (override with `PLAYWRIGHT_BASE_URL`).
- If you want to run against system Chrome instead of Playwright-downloaded browsers:

```bash
cd frontend
PLAYWRIGHT_CHANNEL=chrome pnpm test:e2e
```

## Manual QA checklist (release confidence)

### Navigation / shell
- Login works.
- Admin area loads.
- Connectors page is available at `/connectors`.

### Cases
- Create case.
- Add observables.
- Duplicate case (with confirm) works.
- Tabs rail scrolls (no overflow).

### Alerts
- Create alert.
- Bind to case.
- Alert details render correctly.

### Forum
- Thread list -> open thread: no blank/black screen.
- Post message + attachment.
- "Send via connector" flow works.
- Manual sync pulls replies.

### Communications
- Create comm thread for Slack/Outlook.
- Send message.
- "Sync replies" updates thread + shows last synced meta.

### Connectors / Hub
- Create connector.
- Create method.
- Run execution, open drawer and inspect attempts/events/result.

### AI identity
- Run AI agent.
- AI-authored items display as AI agent, not as a real user.

## Triage (when something fails)

- Backend logs:

```bash
docker compose logs -n 200 backend
```

- Frontend logs:

```bash
docker compose logs -n 200 frontend
```

- Re-run a single test file:

```bash
cd frontend
pnpm vitest run path/to.test.tsx
```

