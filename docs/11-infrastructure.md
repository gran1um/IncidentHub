# Инфраструктура и деплой

## 1. Compose stack

Основной файл:

- `./docker-compose.yml`

Сервисы:

- `postgres`
- `redis`
- `kafka`
- `minio` (S3-compatible)
- `elasticsearch`
- `otel-collector`
- `backend`
- `frontend`
- `prometheus`
- `grafana`

## 2. Healthcheck политика

В проекте применяется ограничение `timeout: 5s` для healthchecks критичных сервисов:

- postgres
- redis
- kafka
- elasticsearch

Это синхронизировано с API health module timeout (5 секунд).

## 3. Volumes

Используемые persistent volumes:

- `postgres_data_v3`
- `redis_data`
- `kafka_data`
- `elastic_data_v3`
- `minio_data`
- `prometheus_data`
- `grafana_data`

## 4. Backend runtime dependencies

Backend depends_on:

- postgres (healthy)
- redis (healthy)
- elasticsearch (healthy)
- kafka (healthy)
- minio (started)

Workflow engine runtime:

- `WORKFLOW_ENGINE=internal` — built-in workflow runtime (node executors внутри backend)

## 5. S3 артефакты

MinIO используется как локальный S3 backend:

- endpoint `http://minio:9000`
- console `http://localhost:9001`
- backend может auto-create bucket.

## 6. AI runtime

AI runtime работает через внешний LLM endpoint:

- `AI_PROVIDER` и `AI_ENDPOINT` задаются в `.env`.
- backend не поднимает локальный контейнер модели.
- поддерживаются OpenAI-compatible и Ollama-совместимые удаленные endpoint.

## 7. Горизонтальное масштабирование

Backend масштабируется горизонтально:

- `docker compose up -d --scale backend=3`

Требования для корректного масштаба:

- shared PostgreSQL, Redis, Elasticsearch, S3;
- отсутствие stateful business logic в памяти backend.

## 8. Что масштабируется отдельно

Согласно текущему дизайну:

- backend масштабируется независимо;
- postgres/elasticsearch масштабируются отдельно инженерной командой при необходимости.

## 9. Локальный запуск

- `cp .env.example .env`
- `docker compose up -d --build`

Для AI:

- задать внешний endpoint в `AI_ENDPOINT` (например `http://host:8001/v1/chat/completions`);
- при необходимости задать `AI_PROVIDER=openai-compatible` и `AI_MODEL`;
- `OPENAI_API_KEY`/`AI_API_KEY` можно оставить пустым для gateway без auth.

Порты:

- Frontend: `5173`
- Backend API (через frontend/nginx proxy)
- PostgreSQL: `5432`
- Redis: `6379`
- Elastic: `9200`
- Kafka: `9092/29092`
- MinIO: `9000/9001`
- Prometheus: `9090`
- Grafana: `3000`

## 10. Production hardening checklist

- сменить дефолтные секреты и пароли;
- включить TLS для внешних endpoints;
- закрыть прямой доступ к internal сервисам;
- добавить backup/retention policy для Postgres и S3;
- включить index lifecycle policy для Elasticsearch;
- добавить alerting для health/latency/error budget.
- мониторить `workflow_engine` модуль в `/api/v1/system/health`.
