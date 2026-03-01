# IncidentHub Load Test Module

This module benchmarks tenant-scoped API read/write throughput and estimates sustainable RPS for each route profile.

## What is measured

- Read routes:
  - `GET /api/v1/me`
  - `GET /api/v1/alerts`
  - `GET /api/v1/cases`
  - `GET /api/v1/cases/{fixture_case_id}`
  - `GET /api/v1/search?q=<probe>`
- Write routes:
  - `POST /api/v1/alerts`
  - `POST /api/v1/cases` (optional)

Async compatibility:

- If `POST /api/v1/cases` returns async envelope (`202 Accepted`, `operation_id`, `resource_id`), fixture setup automatically polls `GET /api/v1/operations/{operation_id}` until `status=done`.
- If operation status becomes `failed`, loadtest aborts fixture initialization with operation error details.

## Run

From repository root:

```bash
make loadtest ARGS="-base-url http://localhost:5173 -email admin@incidenthub.local -password 'ChangeMeNow123!'"
```

Or directly:

```bash
cd backend
go run ./tools/loadtest -base-url http://localhost:5173
```

## Key options

- `-base-url` API base URL (default: `http://localhost:8080`)
- `-email`, `-password` credentials
- `-tenant-id` explicit tenant override
- `-start-rps`, `-step-rps`, `-max-rps`
- `-step-duration`
- `-workers`
- `-error-threshold`
- `-p95-threshold`
- `-output` report JSON file path

## Output

For each run two files are written:

- JSON: full raw metrics per scenario/step
- Markdown: summarized report table

Default location:

- `backend/loadtest/results/report-<UTC timestamp>.json`
- `backend/loadtest/results/report-<UTC timestamp>.md`
