# Workflow Studio: Node Extension Guide

## Where nodes are declared

- Frontend node modules live in folders:
  - `frontend/src/features/workflow-studio/nodes/<node-id>/node.ts`
- Node registry is assembled automatically via `import.meta.glob(...)` in:
  - `frontend/src/features/workflow-studio/nodes/index.ts`
- Compatibility re-export:
  - `frontend/src/features/workflow-studio/node-library.ts`
- Node visual map (icons/colors in library + canvas):
  - `frontend/src/features/workflow-studio/node-visuals.ts`
- Every module with `export const node = { ... }` appears automatically in `Block Library`.

## Add a new node

1. Create folder `frontend/src/features/workflow-studio/nodes/<your-node-id>/`.
2. Add `node.ts` and export `node` manifest:
   - `id` (stable key, must match backend executor type)
   - `title`
   - `description`
   - `category`
   - `defaultConfig`
   - optional `fields` for Inspector rendering
3. Open Workflow Studio and use Block Library:
   - click to insert a node
   - or drag from library and drop onto canvas
4. (Optional) Add custom icon/tone in `node-visuals.ts` for your node `id`.
   - if no explicit mapping exists, fallback icon is taken from `category`.

## Inspector fields (no page edits required)

- Inspector reads `node.fields` schema and renders controls dynamically.
- Supported field types:
  - `text`, `textarea`, `number`, `switch`, `select`, `connector`, `connector_method`, `json`
- Saved config is persisted to `definition.nodes[].config`.

## Runtime behavior today

- Backend test-run now executes real node handlers (not simulation only):
  - runtime core: `backend/internal/workflow/`
  - built-in node executors: `backend/internal/workflow/nodes/`
- Built-in executable nodes include:
  - `trigger`, `condition`, `delay`, `transform`
  - `switch`, `scheduler`, `aggregate`
  - `ioc_extract`, `watchlist_match`, `mitre_map`, `risk_score`, `containment_decision`
  - `redis`, `cassandra_query`, `s3_object`
  - `python_code`
  - `postgres_query` (read-only SQL)
  - `telegram_send` (real Telegram outbound send through connector)

## Add backend execution for a new node

1. Add executor in `backend/internal/workflow/nodes/`.
2. Register it in `NewBuiltinSet(...)` (`backend/internal/workflow/nodes/builtin.go`).
3. Ensure node `Type()` matches frontend `node.id`.
4. Add/extend tests in `backend/internal/api/handler_workflows_test.go`.
