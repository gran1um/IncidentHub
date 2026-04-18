# Observability: метрики, логи, трейсинг, health

## 1. Компоненты наблюдаемости

- Prometheus scraping backend `/metrics`
- Grafana datasource provisioning
- OTel collector (trace pipeline)
- Structured logging (JSON)
- API/system health endpoints
- service degradation alerting (tenant rules -> notifications -> Telegram)

## 2. Метрики backend

Реализация: `/backend/internal/metrics/metrics.go`

Экспортируемые метрики:

- `incidenthub_http_requests_total{method,path,status}`
- `incidenthub_http_request_duration_seconds{method,path}`
- `incidenthub_auth_attempts_total{result}`
- `incidenthub_access_denied_total{reason}`
- `incidenthub_module_operations_total{module,operation,status}`
- `incidenthub_module_operation_duration_seconds{module,operation}`
- `incidenthub_module_health_status{module}` (`ok=1`, `disabled=0`, `error=-1`)
- `incidenthub_module_health_response_ms{module}`

Также ведется in-process счетчик API запросов за 24h для `system/resources`.
Дополнительно хранится in-process rolling snapshot модульных операций (RPS + avg latency) для service alert evaluator.

Покрытые модульные операции включают `api`, `database`, `cache`, `search`, `storage`, `ai`, `async_ops`, `connectors_inbound`, `connectors_outbound`, `forumproxy`.

## 3. Prometheus

Конфиг:

- `/deploy/prometheus/prometheus.yml`

Scrape targets:

- `backend:8080` (`/metrics`)
- `otel-collector:8888`

## 4. Grafana

Provisioning:

- datasource `Prometheus` из `deploy/grafana/provisioning/datasources/datasources.yaml`
- dashboard provider `deploy/grafana/provisioning/dashboards/dashboards.yaml`
- готовый dashboard `deploy/grafana/provisioning/dashboards/incidenthub-reliability.json`

Ключевые панели:

- API throughput / error-rate / p95 latency;
- module operations throughput / errors / p95 latency;
- module health status + response time;
- auth failures + access denied;
- reliability панель для async/kafka + workflow/forum pipeline.

## 5. Трейсинг

Инициализация:

- `/backend/internal/tracing/init.go`

Используется anyinfra tracer:

- environment-based collector profile;
- service metadata (tenant key/service key/log group/system);
- интеграция с OTel.

Трейсы создаются на HTTP-слое и на уровне модульных операций (`component.module`, `component.operation`) для основных сервисов (AI, storage, connectors, async ops, search, cache, database).

## 6. Логирование

Инициализация:

- `/backend/internal/logger/logger.go`

Свойства:

- JSON format;
- configurable log level;
- связка с anyinfra logster.

## 7. Health endpoints

### 7.1 `/healthz`

Базовый API health.

### 7.2 `/api/v1/system/health`

Проверяет модули:

- `api`
- `postgres`
- `redis`
- `elasticsearch`
- `s3`
- `ai_model`
- `async_ops`
- `workflow_engine`
- `forum_proxy`

Формат каждого модуля:

- `status: ok|error|disabled`
- `responseMs`
- `message` (при ошибке)

Важно: каждый модульный check ограничен `context.WithTimeout(..., 5s)`.

### 7.3 `/api/v1/system/resources`

Для админки возвращает:

- module health
- host resources (cpu/memory/disk)
- postgres pool stats
- tenant resource stats
- dashboard stats
- uptime

## 8. SLO/эксплуатационные рекомендации

- контролировать p95/p99 latency по `http_request_duration_seconds`;
- отслеживать auth failure spikes;
- строить alerting по `access_denied_total` и health status;
- для scale-out опираться на Prometheus aggregation, а не in-process summary.
- для tenant-level Telegram alerting использовать `catalog/service_alert_rules` (метрики `low_rps` / `high_latency`) и настраивать cooldown, чтобы исключить alert spam.
