# Connectors and Connector Hub

Connector Hub is the generic outbound integration layer for IncidentHub. It is used both by analysts and by AI agents through the same execution path.

## Product model

There are three core entities:

- `outbound_connectors` — connector instances with transport-specific config
- `connector_methods` — reusable actions bound to a connector instance
- `connector_hub_executions` — durable execution records with attempts and event log

A method is transport-agnostic from the UI point of view: the page only knows which observable types it applies to and which connector/method should be executed. This lets us add new connector types without hardcoding page-specific buttons.

## Observable-aware method matching

`connector_methods` can declare `observable_types`.

Examples:

- `ip`
- `domain`
- `host`
- `url`
- `email`
- `hash`
- `user_agent`
- `user`
- `process`
- `file`

The frontend normalizes aliases into canonical keys. For example:

- `ipv4`, `ipv6`, `ip address` -> `ip`
- `fqdn` -> `domain`
- `hostname`, `computer` -> `host`
- `sha256`, `md5` -> `hash`

That normalization lives in `./frontend/src/lib/connectors.ts`.

## Analyst flow in cases and alerts

### Cases

On `/cases/:id` each observable row can open a connector dropdown.
Only methods whose `observable_types` match the observable are shown.

Execution payloads are sent through:

- `POST /api/v1/connectors/hub/execute`

Important metadata:

- `case_id`
- `observable_id`
- `observable_type`
- `metadata.source = "case_observable"`

### Alerts

On `/alerts/:id` the page derives observables from the alert content (IP, host, process, URL, email, hash) and exposes the same generic connector flow.

Important metadata:

- `alert_id`
- `observable_id`
- `observable_type`
- `metadata.source = "alert_observable"`

## Durable runtime

Connector Hub is no longer treated as a fire-and-forget UI helper.
The runtime is durable and exposes execution detail, attempts, retries, cancel/restart, and an append-only event log.

Key API surface:

- `POST /api/v1/connectors/hub/execute`
- `GET /api/v1/connectors/hub/executions`
- `GET /api/v1/connectors/hub/executions/:id`
- `GET /api/v1/connectors/hub/executions/:id/events`
- `POST /api/v1/connectors/hub/executions/:id/retry`
- `POST /api/v1/connectors/hub/executions/:id/restart`
- `POST /api/v1/connectors/hub/executions/:id/cancel`

UI entry points:

- case detail drawer
- alert detail drawer
- AI workload trace links into connector executions
- administration connectors area

## Catalog APIs

The following catalog kinds are active for connectors:

- `outbound_connectors`
- `connector_methods`

`connector_methods` is not deprecated anymore and is used by the generic observable flow.

## v1 scope

Version 1 is outbound-first.
Inbound ingestion connectors are removed from the public v1 operator flow and are not available in the main UI/API surface.

Current operator-facing connector work should assume:

- outbound connectors are the supported path
- connector methods are the contract for analyst/agent actions
- Connector Hub executions are the source of truth for runtime visibility

## Main code locations

Backend:

- `./backend/internal/api/handler_connector_hub.go`
- `./backend/internal/connectors/outbound`
- `./backend/internal/api/handler_ai_agents.go`
- `./backend/internal/api/handler.go`

Frontend:

- `./frontend/src/lib/api/connectors.ts`
- `./frontend/src/lib/connectors.ts`
- `./frontend/src/pages/case-detail.tsx`
- `./frontend/src/pages/alert-detail.tsx`
- `./frontend/src/pages/administration.tsx`
