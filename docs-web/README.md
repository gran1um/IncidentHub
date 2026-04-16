# IncidentHub Docs Web (Docusaurus)

Web-портал документации для IncidentHub на Docusaurus.

## Запуск

```bash
pnpm install
pnpm --filter incidenthub-docs start
```

По умолчанию стартует на `http://localhost:3200`.

## Сборка

```bash
pnpm --filter incidenthub-docs build
pnpm --filter incidenthub-docs serve
```

## Docker Compose

В общем compose добавлен сервис `docs-web`.

```bash
docker compose up -d --build docs-web
```

Открыть: `http://localhost:3200`

## OpenAPI sync

Перед `start/build` автоматически копируется актуальная спецификация:

- source: `backend/internal/api/openapi.json`
- target: `docs-web/static/openapi.json`

## Docs sync

Перед `start/build` автоматически копируются, санитизируются и раскладываются по разделам:

- source: `README.md` и `docs/*.md` (основная документация платформы)
- source: `backend/tools/loadtest/README.md`
- source: `frontend/src/features/workflow-studio/README.md`
- source: `backend/loadtest/results/*.md` (в разделе Appendix / Loadtest Reports)
- target: `docs-web/docs-generated/**`

Разделы в веб-доках:

- Overview
- Product
- Architecture
- Data Layer
- Operations
- References
- Appendix
