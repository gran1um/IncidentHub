# IncidentHub: Краткий каталог функционала

Дата актуализации: 2026-02-28

Ниже перечислен уже реализованный функционал платформы, сгруппированный по разделам, чтобы было понятно, к какой части продукта относится каждая возможность.

## 1. Доступ и безопасность

- [Auth] Логин/рефреш/логаут с сессионной моделью и refresh-cookie.
- [RBAC] Роли: `platform admin`, `tenant admin`, `analyst`, `viewer`.
- [Multitenancy] Tenant-scoped доступ к данным и операциям (изоляция между тенантами).
- [API Tokens] Админские API токены с ограничениями по user-account эндпоинтам.
- [Security] Сессии пользователя: просмотр активных сессий и отзыв других сессий.

## 2. Управление тенантами и пользователями

- [Tenants] Создание/редактирование тенантов (platform admin).
- [Users] Создание, редактирование, удаление пользователей.
- [Profiles] Профиль пользователя, аватар/cover upload.
- [Tenant Users] Tenant-список пользователей для назначения владельцев/исполнителей.
- [Experience] XP/награды/история опыта и performance по пользователю.

## 3. Dashboard и мониторинг SOC-потока

- [Dashboard Stats] Базовые KPI: активные кейсы, алерты за 24ч, resolved today, средний response.
- [Duty] Дежурства: текущая/следующая смена, список on-duty.
- [Livestream] Единая лента изменений по кейсам/алертам/таскам с фильтрами.
- [Metrics Hub] Операционные метрики: open cases/alerts, overdue cases, SLA по критичности, разбивка по статусам/категориям.
- [Custom Metrics] Конструктор пользовательских метрик (create/edit/enable/disable/delete).

## 4. Alerts

- [Alerts CRUD] Создание, просмотр, обновление, удаление алертов.
- [Bulk Ops] Массовые операции: привязка алертов к кейсу, создание кейса из алертов.
- [Pagination/Filters] Пагинация и фильтры в списке алертов.

## 5. Cases

- [Cases CRUD] Полный жизненный цикл кейса (создание/обновление/удаление/просмотр).
- [Case Statuses] Tenant-кастомизация статусов кейса.
- [Case Templates] Создание кейсов из шаблонов.
- [Custom Fields] Пользовательские поля кейса (включая SOAR preset поля).
- [Related Cases] Поиск связанных кейсов по observables.
- [Cross-tenant Share] Шеринг кейса между тенантами как единой сущности.
- [Summary API] Сводка по кейсам для карточек (таски/прогресс/статусы запусков).

## 6. Case Workspace

- [Tasks] Таски внутри кейса (CRUD, назначение, прогресс).
- [Observables] IOC/observable сущности (CRUD, теги, вердикты).
- [Timeline] События кейса и аналитические записи.
- [Pages] Страницы кейса (длинные заметки/артефакты расследования).
- [Attachments] Файлы и изображения, загрузка и скачивание через storage.
- [Comments] Комментарии по кейсу.
- [DataSec Incidents] Отдельный Incident-as-a-Service workspace: регистрация, ping, эскалация, закрытие, меры реагирования, метрики и FAQ первой линии.

## 7. Коммуникации и форум

- [Forum] Треды/посты/вложения для обсуждения расследований.
- [Proxy Communications] Отправка/синхронизация сообщений во внешние каналы через proxy flow.
- [Case Communications] Канал коммуникации в рамках кейса (threads/messages/sync).

## 8. Workflow и автоматизация

- [Workflow Studio UI] Визуальная студия воркфлоу (список, создание, редактирование).
- [Native Builder] Графовый редактор воркфлоу в интерфейсе.
- [Workflow AI Agent] Отдельный чат в Workflow Studio для генерации воркфлоу по промпту.
- [Run/Test] Запуск workflow в режиме `run` и `test-run`.
- [Run History] История запусков workflow (статусы, ошибки, длительность).
- [Node Runtime] Встроенные ноды: trigger/control-flow/transform/aggregate/security(ioc/watchlist/mitre/risk/containment)/telegram/postgres/redis/cassandra/python/s3.

## 9. AI модуль

- [AI Chat] Tenant-scoped чат с AI-агентом.
- [AI Sessions] Сессии чатов: создание, удаление, перестановка.
- [Case AI] Анализ конкретного кейса через AI endpoint.
- [RAG Context] Контекст по данным платформы для AI-ответов.

## 10. Уведомления

- [User Settings] Персональные настройки доставки уведомлений.
- [Admin Bots] Управление Telegram-ботами на уровне тенанта.
- [Delivery Pipeline] Асинхронная доставка уведомлений (Kafka queue + retry/DLQ).
- [Service Alerts] Правила деградации сервиса (low RPS/high latency) с генерацией уведомлений.

## 11. Поиск и данные

- [Global Search] Глобальный поиск по кейсам/алертам/тредам/DataSec инцидентам.
- [PostgreSQL] Основное транзакционное хранилище.
- [Elasticsearch] Поисковый слой для быстрых выборок и AI retrieval.
- [Redis] API cache, rate-limit и ускорение read-сценариев.

## 12. Наблюдаемость и эксплуатация

- [Health] `/healthz` и `/api/v1/system/health`.
- [Resources] `/api/v1/system/resources` для runtime-среза.
- [Metrics] Prometheus `/metrics`.
- [Tracing/Logging] Трассировка операций и аудит действий.
- [Async Ops] Асинхронные CRUD операции с polling статуса (`operations/:id`).

## 13. Frontend UX-слой

- [Layout] Sidebar/topbar/tenant switcher/global search.
- [URL State] Сохранение состояния страниц/фильтров/вкладок в URL.
- [Responsive] Поддержка мобильной верстки для ключевых экранов.
- [i18n] RU/EN локализация.

---

Если нужен отдельный вариант этого списка для инвест-презентации (коротко: бизнес-язык + ценность для SOC), можно сделать вторую версию на 1 страницу.
