# Frontend: архитектура и UX

## 1. Технологический стек

- React 18 + TypeScript + Vite
- Маршрутизация: `wouter`
- Data fetching/cache: `@tanstack/react-query`
- UI primitives: Radix UI + кастомные компоненты
- Иконки: `lucide-react`
- Графики: `recharts`
- Анимации: `framer-motion` + CSS transitions
- Пакетный менеджер: `pnpm`

Ключевые файлы:

- `./frontend/src/App.tsx`
- `./frontend/src/components/layout.tsx`
- `./frontend/src/lib/api.ts`
- `./frontend/src/lib/api/core.ts`
- `./frontend/src/lib/api/*.ts`
- `./frontend/src/lib/i18n.ts`
- `./frontend/src/lib/theme.ts`

## 2. Маршруты и экраны

Основные роуты приложения:

- `/` — Dashboard
- `/alerts` — список алертов
- `/alerts/:id` — карточка алерта
- `/cases` — список кейсов
- `/cases/:id` — workspace кейса
- `/incidents` — DataSec Incident-as-a-Service workspace (tenant-scoped)
- `/antifraud` — anti-fraud workspace (tenant-scoped)
- `/forum` — список форум-тредов
- `/forum/:id` — thread view
- `/templates` — шаблоны и observable types
- `/workflow-studio` — native IncidentHub graph builder + AI workflow assistant
- `/administration` — admin panel
- `/profile` — профиль пользователя
- `/security` — безопасность (пароль и сессии)
- `/users/:id` — профиль пользователя (единый компонент view)
- `/login` — авторизация

Admin-only роуты защищены на уровне frontend (redirect для не-admin).

## 3. Layout и UX-концепция

`AppLayout` реализует:

- левый sidebar с collapse/expand анимацией;
- topbar с tenant switcher, global search и user menu;
- правый AI panel с persisted chat;
- модульный status-блок системы (api/postgres/redis/elasticsearch/s3/ai_model);
- hover/transition states для интерактивных элементов;
- кнопка Ask AI с выделенной визуальной анимацией.

## 4. Состояние и источники данных

### 4.1 Бизнес-данные

Frontend не хранит доменную логику в localStorage. Источник правды:

- backend API;
- react-query cache.

### 4.2 Локальные пользовательские настройки

Используются в local storage:

- тема (`theme-storage`);
- язык (`i18n-storage`);
- UI-переключатели (например скрытие AI-панели).

Это UI/session слой, не хранилище бизнес-сущностей.

## 5. API-клиент и обработка auth

`coreFetch` в `/frontend/src/lib/api/core.ts` делает:

- автоматическую подстановку `Authorization: Bearer` (in-memory access token);
- автоматическую подстановку `X-Tenant-ID`;
- авто-refresh при `401` через `/api/v1/auth/refresh`;
- bootstrap сессии через refresh-cookie (`useSessionBootstrap`) при старте приложения;
- нормализацию ошибок сервера.

## 6. Основные frontend хуки

Теперь API-hooks разбиты по модулям, а `lib/api.ts` работает как barrel-export:

- `lib/api/app.ts` — session/bootstrap и глобальный app-state.
- `lib/api/tenants-users.ts` — tenants/users/profile/xp.
- `lib/api/alerts.ts` — alerts + bulk-операции.
- `lib/api/cases.ts` — cases/statuses/shares/tasks/observables/pages/comments/timeline/AI analyses.
- `lib/api/forum.ts` — forum threads/posts/proxy.
- `lib/api/communications.ts` — case communications + sync.
- `lib/api/catalog.ts` — templates, observable types, categories, workflows, shifts.
- `lib/api/dashboard.ts` — dashboard stats/metrics/duty/health/system resources.
- `lib/api/notifications.ts` — notification bots/settings/rules + user notifications.
- `lib/api/admin.ts` — admin API tokens, rate limits, SOC access policy, search/config/sessions.
- `lib/api/ai.ts` — AI chat sessions/messages + AI agents operations.
- `lib/api/achievements.ts` — achievements/profile badges/bio.
- `lib/api/connectors.ts` — connectors/methods/hub/automations.

Текущая реализация общих helper-функций и `coreFetch` находится в `lib/api/core.ts`; доменные модули переиспользуют этот слой.

## 6.1 Workflow Studio: UX-флоу

В `Workflow Studio` реализован сценарий:

- единый список всех workflow + кнопка `Create workflow`;
- открытие/редактирование существующего workflow;
- block library с авто-подхватом новых node-модулей из `features/workflow-studio/nodes/*`;
- `Enabled/Disabled` и `Published/Draft` переключатели;
- test-run прямо из UI (`Test run`) без авто-сохранения драфта;
- production run (`Run`) для сохраненного workflow без runtime override;
- история запусков в правой панели с последними статусами/ошибками/длительностью;
- сохранение изменений только по кнопке `Save`.
- в editor header всегда отображается текущее имя workflow.
- отдельный AI Agent чат (иконка в header) для генерации workflow по промпту с кнопкой `Apply to canvas`.

Block library подхватывает node manifests автоматически и сейчас включает:

- `trigger`
- `condition`, `switch`, `delay`, `scheduler`
- `transform`, `aggregate`
- `ioc_extract`, `watchlist_match`, `mitre_map`, `risk_score`, `containment_decision`
- `telegram_send`
- `postgres_query`, `redis`, `cassandra_query`, `s3_object`
- `python_code`

Практически все кнопки UI работают через эти hooks и API endpoints.

## 6.2 Administration: SOC Access Policy

Во вкладке администрирования добавлен блок `SOC Access Policy`:

- `Allowed case tags` (comma-separated) — tag-based сегментация кейсов для analyst-role;
- `Max in-work cases per analyst` — лимит одновременно обрабатываемых кейсов (0 = unlimited);
- сохранение через catalog kind `soc_access_policies`;
- policy применяется на backend (не только на уровне UI), поэтому ограничения сохраняются при прямых API-вызовах.

## 6.3 Dashboard: дежурства из API

Верхний блок dashboard использует не только локальный расчет из `shifts`, но и server-side snapshot:

- `useDutyOverview(currentTenantId)` -> `GET /api/v1/duty/overview`;
- показывает:
  - количество активных смен сейчас;
  - время следующей смены;
  - количество аналитиков в следующей смене;
- если API временно пустой, UI автоматически откатывается на fallback-расчет по данным `shifts`, чтобы не ломать экран.

## 6.4 Dashboard: Metrics Hub и Custom Metrics Builder

На dashboard добавлен блок `Operational Metrics`:

- open cases/open alerts/overdue cases;
- SLA по критичности (open/resolved/breached + avg resolution minutes);
- разобранные кейсы по аналитикам;
- распределение кейсов по статусам и категориям.

В этом же блоке доступен `Custom metrics` builder:

- создание метрики по источнику `cases|alerts`;
- выбор measure:
  - `count` (cases/alerts),
  - `avg_resolution_minutes` (cases),
  - `overdue_count` (cases);
- фильтры по `status/severity/category`;
- фильтр по окну `created_within_hours`;
- настройка `overdue_minutes` для overdue метрик;
- edit/enable/disable/delete кастомных метрик из UI.

## 7. i18n

- Поддерживаются `en` и `ru`.
- Все ключи сосредоточены в `/frontend/src/lib/i18n.ts`.
- В UI используется `useT()`.
- Для оставшихся точек есть отдельный аудит: `docs/i18n-audit.md`.

## 8. Тема (light/dark)

Логика темы:

1. при первом входе — системная тема (`prefers-color-scheme`);
2. далее ручное переключение пользователем;
3. выбранное состояние персистится в `theme-storage`.

## 9. Анимации и интерактивность

Применяются на:

- sidebar collapse/expand;
- кнопки и карточки (hover/transition);
- открытие/закрытие panel/dialog;
- AI panel и search modal interactions;
- состояния загрузки (skeleton/spinner/toasts).

## 10. Роль-ориентированный UI

Frontend скрывает недоступные admin разделы для обычных пользователей, но основная защита остается на backend через RBAC middleware.

## 10.1 Case Workspace: Pages

Во вкладке кейса добавлен отдельный пользовательский flow для `Case Pages`:

- tab `Pages` в `/cases/:id`;
- создание long-form страницы (`title`, `body`) прямо из UI;
- список уже созданных страниц с автором и временем создания.

Это закрывает backend-only API `GET/POST /cases/:caseID/pages` и делает функцию доступной аналитикам без ручных API-вызовов.

## 10.2 Case Creation Flow (Templates + Integration Fields)

В диалоге создания кейса (`/cases`) реализован расширенный пользовательский сценарий:

- выбор `Case Template` с предзаполнением полей (title/description/severity/tags/custom fields);
- явный выбор `status` при создании (tenant-specific case statuses);
- `tags` при создании кейса;
- `additional fields` (custom fields) прямо в форме создания;
- обязательная валидация интеграционных полей: `mdm`, `scenario_id`, `incident_date`.

После создания `tags/custom_fields` синхронно сохраняются в `case_meta`, чтобы карточка кейса и фильтры сразу работали консистентно.

## 10.3 Case Workspace: Related Cases Chains

Во вкладке `Overview` на странице кейса добавлен блок `Related Cases`:

- `Active / Recent Chain` — связанные кейсы, которые активны или обновлялись недавно;
- `All-time Related` — связанные кейсы за весь период;
- переход в связанный кейс по клику на карточку.

Источник данных: `GET /api/v1/cases/:caseID/related`. Корреляция строится по совпадающим observables (`type + value`) с учетом tenant-доступа.

## 10.4 Structured SOAR Fields (L309-L323)

Для пользовательского флоу по настраиваемым полям кейса добавлен стандартный набор SOAR-полей:

- `analyst`
- `indicators`
- `status`
- `stages`
- `description`
- `inbound_event`
- `criticality`
- `role_types`

UI-точки, где это доступно:

- `/templates` -> `Case Templates`:
  - кнопка `Add SOAR Fields` добавляет preset в `Default Custom Fields`.
- `/cases` -> диалог создания кейса:
  - кнопка `Add SOAR fields` добавляет preset в `Additional Fields`.
- `/cases/:id` -> вкладка `Overview` -> `Custom Fields`:
  - кнопка `Add SOAR Fields` добавляет недостающие preset-поля прямо в кейс.

Для preset-ключей используются специальные placeholder-ы, чтобы аналитик сразу понимал формат ожидаемых данных.

## 10.5 Case Timeline, Notes, Export, Playbook Results (L324-L335)

Во вкладке `Timeline` на странице `/cases/:id` добавлен полный пользовательский flow:

- аналитика по действиям внутри кейса:
  - общее число действий;
  - среднее время на действие;
  - успешность (по задачам и event outcome);
  - breakdown по статусам/действиям и времени на этапы;
- заметки аналитика:
  - отдельная форма `Analyst Notes`;
  - запись заметки в `case_timeline_events` (`event_type=note`);
  - моментальное отображение в timeline;
- выгрузка кейса:
  - кнопки `Export JSON / CSV / MD`;
  - экспортирует кейс + timeline + tasks + observables + comments + attachments + pages + playbook runs;
- файлы/изображения:
  - во вкладке `Attachments` явная маркировка `Image/File`;
  - для изображений доступен быстрый preview/open;
- результаты выполнения плейбуков:
  - блок `Playbook Run Results` прямо в timeline;
  - показывает связанные с кейсом workflow-runs (status/duration/error/result preview).

## 10.6 Playbook Catalog + Suggested Autofill (L339-L343)

Во вкладке `Timeline` для кейса добавлен рабочий launcher плейбуков:

- список всех доступных включенных playbook (`enabled workflows`) для tenant-а;
- блок `Suggested`:
  - ранжирование плейбуков по контексту кейса (incident type, stage, observables, severity);
  - причину рекомендации видно рядом с плейбуком;
- автозаполнение при выборе плейбука:
  - `node`
  - `host`
  - `indicators`
  - `case_id/case_number` и расширенный `case` context;
- редактируемый JSON input перед запуском;
- запуск плейбука прямо из кейса кнопкой `Run Playbook`.

Это закрывает UX-разрыв: аналитик из кейса сразу видит релевантные playbook и может запускать их без ручной сборки контекста.

## 10.6A Case Page Deep UX (L555-L585)

На странице `/cases/:id` закрыт полный пользовательский контур для задач/обновлений/SLA:

- Description:
  - `Rendered` теперь поддерживает markdown (`headings`, `lists`, `links`, `code`, `images`) + безопасный рендер HTML-вставок с цветным шрифтом;
  - `Raw` сохраняет исходный текст без преобразования.
- Tasks:
  - создание задач с `due date`;
  - режимы цепочки задач: `Manual`, `Semi-automated`, `Automated` (сохранение в `custom_fields.task_chain_mode`);
  - KPI по задачам в кейсе: progress + open/overdue.
- SLA и напоминания:
  - карточка SLA на кейсе (`target minutes`, `due at`, breached/on-track);
  - быстрые напоминания (`+15m / +1h / +24h`) с записью события `event_type=reminder` в timeline.
- Обновления кейса:
  - отдельный блок `Recent case updates` (последние события из timeline).
- Визуальный алгоритм:
  - для выбранного playbook отображается компактный граф workflow (`nodes` + `edges/conditions`) в самом кейсе.
- Выполнения responder-сценариев:
  - отображаются как `workflow run` результаты на таймлайне (единый runtime вместо legacy responder UI).

## 10.6B Additional Case Features (L586-L598)

- Логирование (`L589`):
  - отдельная страница `Livestream` (`/:tenant/activity`) с автообновлением (`3s`) и фильтрами по типам сущностей (`case/alert/task`), assignee и тексту;
  - открытие в новом окне и быстрый переход в карточку сущности из лога.
- Темный/светлый режим (`L591`):
  - глобальный переключатель темы в layout;
  - переключатели темы на экране логина;
  - состояние темы сохраняется между сессиями.
- Поэтапное закрытие + multi-approval (`L593-L596`):
  - в кейсе добавлен блок `Closure Approvals`;
  - аналитики фиксируют апрув кнопкой `Approve`, в timeline пишется `event_type=closure_approval`;
  - UI показывает прогресс апрувов, обязательных апруверов и готовность к финальному закрытию.
- Отображаемость по связанным полям (`L598`):
  - в блоке `Related Cases` добавлен selector `Link by`;
  - поддерживаются режимы: `Observables`, `Incident Type`, `Source`, `Severity`, `Priority`;
  - для non-observable режимов в карточке отображается matched field/value.

## 10.7 Time Chat Window in Case Communications (L347-L350)

Во вкладке `Communications` на странице `/cases/:id` добавлен отдельный пользовательский flow для `Time`:

- отдельный блок `Time Chat Window` над списком тредов;
- выбор `Time connector` и пользователя тенанта из dropdown;
- запуск чата кнопкой `Start Time Chat`:
  - если чат уже существует для выбранного пользователя/коннектора, открывается существующий тред;
  - если чата нет, создается новый thread c `channel=time`;
- история сообщений сохраняется и отображается в правой колонке, как и для остальных коммуникаций;
- если для tenant-а не настроен ни один `Time` connector, показывается явный информер.

## 10.7A Email Templates + Subject Sync + Comments (L617-L622)

Во фронтенде закрыт полный пользовательский flow по коммуникациям кейса:

- `/templates`:
  - добавлен tab `Communication Templates`;
  - создание шаблонов `communication_templates` (name/channel/subject_template/body_template);
  - список существующих шаблонов для tenant-а.
- `/cases/:id` -> `Communications`:
  - при отправке сообщения доступен выбор шаблона (`Template`) и `Template Variables (JSON)`;
  - `subject` редактируется в composer и пробрасывается в send/sync API;
  - `Sync Replies` подтягивает письма из ящика по теме (subject filter), если тема задана.
- `/cases/:id` -> `Comments`:
  - комментарии работают как чат-поток по кейсу (создание/лента/авторы/время).

## 10.7B Connectors + Role Flows + Integration Fields (L630-L678, L702-L709, L755-L765)

Реализованные пользовательские флоу:

- `Administration -> Connectors`:
  - поддержка connector types: `HTTP`, `SQL`, `S3`, `Kafka`, `Redis`, `SMTP`, `LDAP`, `Webhook`, `Syslog`, `Custom`;
  - поддержка auth-стратегий в коннекторах (`api_key`, `bearer`, `basic`) и LDAP/AD-конфигурации;
  - методы коннекторов для SQL/Postgres-подобных сценариев и почтовых интеграций;
  - inbound source profiles для типовых источников: `dlp`, `antifraud`, `sb`, `data_security`.
- `Case workspace` для сотрудника:
  - видит доступные кейсы, может взять в работу (`Assign to me`);
  - может менять статус/поля кейса;
  - может запускать скриптовую автоматизацию из кейса через playbook/workflow launcher;
  - видит историю коммуникаций и отправляет ответные сообщения из `Communications`.
- `Administration + Templates + Workflow Studio` для администратора:
  - настройка новых типов обработки через `Case Templates` и custom fields;
  - настройка коннекторов и методов интеграции;
  - запуск/доработка автоматизаций с Python-узлами и async run history.
- `Dashboard + Service Alerts`:
  - счетчики просроченных кейсов и SLA-breach визуализация;
  - телеграм-канал уведомлений по правилам service alerts.
- Обязательные интеграционные поля при создании кейса:
  - `mdm`, `scenario_id`, `incident_date` (валидация в create-case форме).

## 10.8 Visualizations & Files Applets (L354-L363)

Во вкладке кейса добавлен отдельный tab `Visuals` (`/cases/:id?tab=visuals`) с фиксированным набором фильтруемых applet-блоков:

- графы:
  - `Graph: by source` — распределение observables по источникам (`network`, `authorization`, `other`);
  - `Graph: by observable type` — топ типов observables;
- таблица-апплет детализации observables:
  - source/type/value/verdict/tags;
  - фильтры по source/type и поиску;
  - реактивная связка: выбор строки в applet-е источников/типов или строки observable автоматически фильтрует второстепенные блоки;
- таблица-апплет файлов:
  - разделение на `images` и `files`;
  - фильтр по типу файлов + общий поиск;
  - действия preview/open.
- дополнительные applet-блоки:
  - `source breakdown` (таблица по источникам);
  - `type breakdown` (таблица по типам);
  - оба блока связаны между собой и с основной таблицей observables.

Это закрывает пользовательский сценарий “визуализации + файлы без конструктора, но с фильтрами” прямо в кейсе.

## 10.9 Cases List Advanced Filters (L367-L391)

На странице `/cases` расширен фильтр-панель для аналитиков и тимлидов:

- полнотекстовый include/exclude поиск:
  - `q` (включение),
  - `q_not` (исключение),
  - `search_mode` (`plain|regex|fulltext`),
  - `search_logic` (`all|any`);
- фильтры по контексту кейса:
  - `severity`,
  - `status`,
  - `stage`,
  - `assigned` (`all|assigned|unassigned|mine`),
  - точечный `assignee`,
  - `rule` (по `incidentType`),
  - `inbound_event`,
  - наличие observables (`all|with|without`),
  - `time in work` (минимум часов),
  - `custom field key/value`;
- фильтры по тегам + диапазону дат, сортировка и группировка остаются доступны;
- закрытые кейсы по умолчанию скрыты, подтягиваются кнопкой `Load closed cases` (обратный режим — `Hide closed cases`);
- сортировка поддерживает базовые поля и `CF:*` (любое доступное custom field);
- все параметры сохраняются в URL-state и восстанавливаются после reload/шеринга ссылки.

На странице `/alerts` добавлен такой же режим `search_mode=fulltext` с `search_logic=all|any`:

- backend использует PostgreSQL full-text (`to_tsvector` + `websearch_to_tsquery`);
- в UI режим доступен в селекторах `Search Mode` / `Search Logic`;
- для `fulltext` локальная пост-фильтрация отключена, чтобы результат соответствовал серверной выдаче 1:1.

## 10.10 Cases Cards, Tagging, Team Filters (L395-L436)

На списке `/cases` добавлены доработки карточек и тегирования, закрывающие следующий блок требований:

- расширенный внешний вид карточки кейса:
  - название;
  - assignee;
  - статус и критичность;
  - дата создания;
  - `time in work` (часы);
  - количество непустых значений в `custom fields`;
  - количество `open/closed` задач;
  - прогресс-бар выполнения задач;
  - статусы автоматизаций/респондера (`queued/running/completed/failed`);
- summary-данные карточек загружаются отдельным API:
  - `GET /api/v1/cases/summary?case_ids=<id1,id2,...>`;
- теги:
  - панель `All Tags` (полный список тегов на текущей выборке + count);
  - клик по тегу включает/исключает его из фильтра;
  - цветовая настройка тега через встроенный color picker (persisted в local storage);
- фильтрация по командам:
  - добавлен `Team` filter в панели списка кейсов.

## 10.11 Livestream + Manual Create + Visible Fields (L437-L454)

Закрыт блок UX-требований по live-активности, ручному созданию и настройке отображения:

- Livestream:
  - новая страница `/activity` с live-лентой изменений по `case/alert/task`;
  - фильтры в ленте: `search`, `assignee`, `entity types`, `limit`;
  - автообновление (polling, 3s) через API;
  - переходы из элемента ленты в сущность (`/cases/:id`, `/alerts/:id`, либо кейс задачи);
  - лента встроена на `/dashboard` как виджет с настройкой расположения (`top|bottom`) и кнопкой открытия в отдельном окне.
- Ручное создание:
  - `Alerts` page: кнопка `New Alert` + модальное окно полноценного ручного создания alert;
  - `Cases` page: кнопка `New Task` + модальное окно ручного создания task для выбранного кейса;
  - ручное создание case осталось на `Cases` page и работает в одном UX-наборе с созданием task.
- Выбор отображаемых полей:
  - на `Cases` page добавлен `Fields` selector;
  - можно включать/выключать базовые поля карточки (`id/status/tags/assignee/created/work time/rule/source/discussion/task progress/responder statuses`);
  - поддержаны поля из `customFields` кейса (`CF: ...`);
- выбранные поля сохраняются в URL-state (`fields`) и `localStorage` для восстановления после reload.

## 10.12 Case Copy + Live Refresh + Conflict Guard (L611-L616)

На странице `/cases/:id` закрыт пользовательский контур работы с кейсом при конкурентной активности:

- в header actions добавлена кнопка `Copy` (`button-copy-case`);
- copy создает новый case на backend (`POST /api/v1/cases/:caseID/copy`) и открывает его сразу в UI;
- карточка кейса автообновляется polling-ом (`useCase(..., { refetchInterval: 5000 })`);
- edit dialog и inline edit отправляют `expected_updated_at`:
  - при конфликте версии backend возвращает `409` вместо silent overwrite;
  - пользователь видит ошибку и может перечитать актуальное состояние.

## 10.13 Массовые операции кейсов и алертов (L172-L186)

На страницах `/cases` и `/alerts` закрыт пользовательский bulk-flow для выделенных элементов:

- изменение статуса выбранных сущностей;
- закрытие выбранных сущностей одной кнопкой;
- добавление тега для выбранных сущностей;
- удаление выбранных сущностей (подтверждение через диалог);
- для алертов дополнительно сохранены существующие действия `Link to Case` и `Create Case`.

Технически важно:

- для `alerts` теги сохраняются через `catalog/alert_meta` (tenant-scoped), чтобы bulk-тегирование было консистентным с карточками и списком;
- для `cases` теги и прочие meta-поля продолжают использовать `case_meta`.

## 11. Тестирование frontend

Текущий основной e2e smoke уровень:

- `./frontend/tests/smoke.sh`

Smoke проверяет критичные бизнес-флоу:

- auth sessions/revoke;
- alerts/cases link/create from alerts;
- case statuses customization;
- forum uniqueness and proxy send;
- case communications (create/send/sync/thread view);
- AI ask + history + case analysis;
- system health/resources.
