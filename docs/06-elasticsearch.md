# Elasticsearch: индексирование и поиск

## 1. Зачем используется Elasticsearch

Elasticsearch в IncidentHub нужен для:

- глобального поиска по SOC данным;
- предоставления retrieval-контекста для AI-ассистента;
- быстрого кросс-сущностного поиска (alerts/cases/observables/forum).

## 2. Индексная стратегия

Индексы формируются как:

- `<ELASTIC_INDEX_PREFIX>-alerts`
- `<ELASTIC_INDEX_PREFIX>-cases`
- `<ELASTIC_INDEX_PREFIX>-observables`
- `<ELASTIC_INDEX_PREFIX>-forum_threads`
- и другие kind-специфичные индексы при индексации

По умолчанию `ELASTIC_INDEX_PREFIX=incidenthub`.

## 3. Где происходит индексация

Индексация вызывается из backend в ключевых точках:

- создание/обновление alerts;
- создание/обновление cases;
- создание observables;
- inbound worker при создании alert;
- отдельные доменные операции, где важен поиск.

Метод:

- `search.Client.IndexDocument(ctx, kind, id, doc)`

## 4. Поисковый запрос

`search.Client.Search()` использует `multi_match` + `phrase_prefix` по полям:

- `title^4`
- `description^2`
- `source^2`
- `name^2`
- `body`
- `value`

Operator: `and` для более точного совпадения.

## 5. Tenant-изоляция в поиске

Текущая реализация:

- запрос выполняется по общим индексам;
- далее hit-фильтрация в backend по `tenant_id` из `_source`.

Плюс:

- проще управление индексами;
- меньше operational overhead.

Минус:

- часть нерелевантных hits может быть отфильтрована постфактум;
- при росте объема возможна потеря эффективности.

## 6. Рекомендация по multi-tenant индексам

Есть два рабочих подхода:

1. Общие индексы + строгий tenant filter (текущий подход).
2. Индексы per tenant (например `incidenthub-<tenant>-alerts`).

Текущий код использует вариант 1. Для очень крупных tenant-ов и требований по жесткой физической сегрегации можно переходить к варианту 2 через алиасы/rollover.

## 7. Роль в AI

AI service делает retrieval из Elastic по tenant query:

- источник для `Ask`;
- источник для `AnalyzeCase`;
- возвращаемые sources отображаются в UI и сохраняются в history.

## 8. Health и эксплуатация

- `Ping()` проверяет доступность Elastic через `Info`.
- В `/api/v1/system/health` модуль `elasticsearch` имеет статус `ok|error|disabled`.
- Проверка модуля ограничена 5-секундным timeout.

## 9. Ограничения и best practices

- нет явной схемы mappings/templates в коде (используются dynamic mappings);
- при прод-росте желательно ввести index templates и нормализованные поля;
- для аналитики лучше отдельные pipeline и lifecycle policy.
