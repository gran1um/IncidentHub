# IncidentHub Documentation

Дата актуализации: 2026-03-05

Этот раздел содержит детальную документацию по текущей реализации платформы IncidentHub в репозитории.

## Содержание

1. [Функциональная спецификация](./01-product-functional-spec.md)
2. [Frontend: архитектура и UX](./02-frontend.md)
3. [Backend: архитектура и доменные модули](./03-backend.md)
4. [API: карта маршрутов и контрактов](./04-api-reference.md)
5. [PostgreSQL: схема, индексы, ограничения](./05-postgresql-schema.md)
6. [Elasticsearch: индексирование и поиск](./06-elasticsearch.md)
7. [Redis: кеш API, инвалидация, rate-limit](./07-redis.md)
8. [Connectors: Telegram communications only](./08-connectors.md)
9. [AI подсистема: чат и анализ кейса](./09-ai.md)
10. [Observability: метрики, логи, трейсинг, health](./10-observability.md)
11. [Инфраструктура и деплой](./11-infrastructure.md)
12. [Безопасность и мультитенантность](./12-security.md)
13. [Тестирование и контроль качества](./13-testing.md)
14. [ENV-конфигурация](./14-env-reference.md)
15. [Краткий каталог функционала платформы](./16-platform-functional-inventory.md)
16. [LikeC4 архитектура](./architecture.likec4)
17. [i18n аудит фронтенда](./i18n-audit.md)
18. [Экранное ТЗ для дизайнера](./18-designer-screen-spec.md)
19. [План глобальной переработки frontend под Figma](./19-frontend-redesign-implementation-plan.md)
20. [AI Agent Queue и автономное расследование](./20-ai-agent-queue-investigation.md)

## Источники правды

- Код backend: `./backend`
- Код frontend: `./frontend`
- Миграции БД: `./backend/migrations`
- Infra Compose: `./docker-compose.yml`

Документация ниже описывает именно реализованное состояние кода, включая ограничения и текущие технические компромиссы.

## Проверка LikeC4

- `pnpm dlx likec4 validate docs`
- `pnpm dlx likec4 build docs`

## Web-портал документации (Docusaurus)

- Папка проекта: `./docs-web`
- Dev: `pnpm docs:web:dev`
- Build: `pnpm docs:web:build`

OpenAPI для web-доков берется из `./backend/internal/api/openapi.json` и автоматически синхронизируется в `docs-web/static/openapi.json` перед `start/build`.
