# IncidentHub Load Test Report (2026-02-18)

## Scope

- Date (UTC): 2026-02-18
- Target: IncidentHub API via frontend entrypoint `http://localhost:5173`
- Auth user: `admin@incidenthub.local`
- Tenant: `d4685c8e-df14-4131-be43-b281ee2ff7be`
- Queue mode: `ASYNC_OPS_ENABLED=false` (sync CRUD path)
- Tool: `backend/tools/loadtest`
- Pass criteria:
  - `error_rate < 2%`
  - `p95 < 700ms`
  - achieved RPS >= 90% of target

## Executed Profiles

1. Baseline
   - start/step/max: `20/20/80 RPS`
   - step duration: `6s`
   - workers: `64`
   - total requests: `8,400`

2. Normal
   - start/step/max: `50/50/250 RPS`
   - step duration: `8s`
   - workers: `192`
   - total requests: `41,996`

3. Stress
   - start/step/max: `100/100/400 RPS`
   - step duration: `6s`
   - workers: `256`
   - total requests: `36,594`

Total generated traffic: `86,990` requests.

## Summary

- Baseline: 7/7 scenarios passed up to `80 RPS`.
- Normal: 7/7 scenarios passed up to `250 RPS`.
- Stress: 6/7 scenarios passed up to `400 RPS`.
- Bottleneck found: `GET /api/v1/search?q=loadtest` failed already at `100 RPS` (p95 breach only).

## Route-Level Result (Max Sustainable RPS)

- `GET /api/v1/me`: `400`
- `GET /api/v1/alerts`: `400`
- `GET /api/v1/cases`: `400`
- `GET /api/v1/cases/{fixture_case_id}`: `400`
- `GET /api/v1/search?q=loadtest`: `0` (threshold fail at first stress step)
- `POST /api/v1/alerts`: `400`
- `POST /api/v1/cases`: `400`

## Latency / Errors at Top Step

Stress profile (highest tested point per scenario):

- `GET /api/v1/me` @400: p95 `2.162ms`, p99 `3.084ms`, errors `0.00%`
- `GET /api/v1/alerts` @400: p95 `2.603ms`, p99 `3.662ms`, errors `0.00%`
- `GET /api/v1/cases` @400: p95 `2.899ms`, p99 `3.796ms`, errors `0.00%`
- `GET /api/v1/cases/{fixture_case_id}` @400: p95 `2.185ms`, p99 `4.572ms`, errors `0.00%`
- `POST /api/v1/alerts` @400: p95 `7.504ms`, p99 `49.736ms`, errors `0.00%`
- `POST /api/v1/cases` @400: p95 `5.489ms`, p99 `11.877ms`, errors `0.0417%`

Failed scenario:

- `GET /api/v1/search?q=loadtest` @100:
  - achieved RPS: `99.93`
  - success rate: `100%`
  - error rate: `0%`
  - p95: `1343.198ms`
  - fail reason: `p95 1.343198292s > threshold 700ms`

## Analysis

The system handles read/write operational routes very well in current test bounds (up to 400 RPS per route profile) with low error rate and low latency.

The only clear performance issue is search under load with a "no-hit" query. In code path, when search returns empty result, fallback performs list-and-filter operations for alerts/cases/threads, which is substantially heavier and explains high p95:

- `backend/internal/api/handler_forum_catalog.go` (search fallback branches around lines 638-684)

## Recommendations

1. Optimize search fallback path:
   - avoid full fallback scans for high-QPS requests;
   - short-circuit with capped/cheap fallback or return empty quickly when ES enabled.
2. Add cache for repetitive no-hit probes on `/api/v1/search`.
3. Add dedicated load scenario for:
   - hit queries (indexed term),
   - no-hit queries (current bottleneck),
   - mixed 80/20 traffic.
4. Run next capacity campaign with higher ceilings (`600-1000 RPS`) for non-search routes, because limit was not reached at `400`.
5. For purer backend capacity numbers, run same campaign directly against backend service endpoint (without frontend proxy hop).

## Artifacts

- `backend/loadtest/results/baseline-20260218.json`
- `backend/loadtest/results/baseline-20260218.md`
- `backend/loadtest/results/normal-20260218.json`
- `backend/loadtest/results/normal-20260218.md`
- `backend/loadtest/results/stress-20260218.json`
- `backend/loadtest/results/stress-20260218.md`

