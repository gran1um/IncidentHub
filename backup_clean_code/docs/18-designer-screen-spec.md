# IncidentHub: экранное ТЗ для дизайнера

Дата актуализации: 2026-03-01

Документ описывает текущий визуальный функционал продукта по экранам и связям между ними. Это UX/UI-спецификация для дизайна поверх существующей логики (без изменения бизнес-процессов backend).

## 1. Рамки и принципы

- Источник: фактически реализованные страницы frontend (`/frontend/src/pages/*`, `/frontend/src/components/layout.tsx`).
- Мультитенантность обязательна в каждом ключевом экране: рабочие URL имеют формат `/:tenantSlug/...`.
- Роли влияют на доступ к меню и экранам:
  - `platform admin`: полный доступ + переключение tenant сверху.
  - `tenant admin`: доступ к рабочим экранам + Administration.
  - `analyst`: доступ к рабочим экранам без Administration.
  - `viewer`: ограниченный набор (dashboard/profile/security).
- Состояния (loading/empty/error/success) должны иметь отдельные визуальные состояния, а не только “happy path”.
- Для длинных значений используется паттерн `ellipsis + tooltip только при truncation`.

## 2. Глобальный каркас приложения

## 2.1 Левое меню (Sidebar)

- Логотип/бренд + collapse (desktop) / drawer (mobile).
- Пункты навигации:
  - `Dashboard`
  - `Activity`
  - `Alerts`
  - `Cases`
  - `Incidents`
  - `AntiFraud`
  - `Forum`
  - `Templates`
  - `Workflow Studio`
  - `Administration` (только admin-роли)
- Счетчики непросмотренных/объектов возле `Alerts` и `Cases`.
- Внизу: `Support` и компактный `Status`-виджет (overall + модули с latency/цветом).

## 2.2 Верхняя панель (Top Bar)

- Tenant switcher:
  - platform admin: dropdown выбора tenant.
  - остальные роли: read-only badge текущего tenant.
- Глобальный поиск в строке (popover под инпутом, не модалка по центру):
  - результаты: alerts, cases, forum threads, datasec incidents, antifraud events/incidents.
  - горячая клавиша `Cmd/Ctrl + K`.
- `Quick Action` dropdown:
  - New incident (быстрое создание кейса),
  - Invite analyst,
  - Run playbook/workflow.
- Notifications popover.
- Профиль-меню: profile, security, язык, тема, logout.

## 2.3 Глобальный AI-помощник

- FAB-кнопка справа снизу (иконка Sparkles, анимированная обводка).
- Кнопка скрытия/показа панели.
- Диалог AI:
  - слева список чатов (создание, удаление, drag-and-drop reorder),
  - справа переписка, блок рекомендаций, поле ввода снизу,
  - при отправке: immediate user message + thinking state + кнопка Stop.

## 2.4 Общие UX-паттерны для всех экранов

- Карточки со скруглениями, четкая иерархия через `Card` + `Badge`.
- Опасные действия только через подтверждение (`AlertDialog`), без системных `alert()`.
- Пагинация: верх/низ списка там, где есть длинные листинги.
- Размер страницы: `10 / 30 / 50 / 100` (или `10 / 30 / 50` в Workflow list).
- Состояние фильтров и вкладок сохраняется в URL query для reload-safe UX.

## 3. Карта экранов (роуты)

| Роут | Экран | Основные роли |
|---|---|---|
| `/login` | Login | все |
| `/:tenantSlug/dashboard` | Dashboard | все авторизованные |
| `/:tenantSlug/activity` | Activity Stream | analyst+ |
| `/:tenantSlug/alerts` | Alerts list | analyst+ |
| `/:tenantSlug/alerts/:id` | Alert detail | analyst+ |
| `/:tenantSlug/cases` | Cases list | analyst+ |
| `/:tenantSlug/cases/:id` | Case detail workspace | analyst+ |
| `/:tenantSlug/incidents` | DataSec incidents | analyst+ |
| `/:tenantSlug/antifraud` | AntiFraud | analyst+ |
| `/:tenantSlug/forum` | Forum threads list | analyst+ |
| `/:tenantSlug/forum/:id` | Forum thread | analyst+ |
| `/:tenantSlug/templates` | Templates | analyst+ |
| `/:tenantSlug/workflow-studio` | Workflow Studio | analyst+ |
| `/:tenantSlug/administration` | Administration | tenant/platform admin |
| `/:tenantSlug/profile` | My profile | все авторизованные |
| `/:tenantSlug/security` | Security settings | все авторизованные |
| `/:tenantSlug/users/:id` | User profile | analyst+ |

## 4. Экранные спецификации

## 4.1 Login

- Назначение: вход пользователя.
- Блоки:
  - бренд + subtitle,
  - email/password,
  - переключатели theme/light-dark и RU/EN.
- Состояния: loading, auth error.
- Переход: после логина -> `/:tenantSlug/dashboard`.

## 4.2 Dashboard

- Назначение: обзор SOC-операций.
- Блоки:
  - KPI карточки (`active cases`, `alerts 24h`, `resolved`, `avg response`),
  - On-duty card (аватары дежурных, current/next shift),
  - Duty schedule modal (календарь смен, добавление/удаление смен),
  - Operational Metrics (SLA, распределения, custom metrics builder),
  - Livestream widget (поиск/размещение top/bottom),
  - график `Case Resolution by Severity` (week/month),
  - `Top analysts`.
- Переходы:
  - аватар аналитика -> user profile,
  - элементы livestream -> case/alert detail.

## 4.3 Activity Stream

- Назначение: живая лента изменений.
- Блоки:
  - фильтры: search, assignee, limit, type toggles (case/alert/task),
  - список карточек событий с auto-refresh `3s`,
  - CTA `Open` на связанный объект.
- Переходы: event -> alert/case detail.

## 4.4 Alerts list

- Назначение: triage и массовая обработка алертов.
- Блоки:
  - фильтры: search (plain/regex/fulltext, all/any), severity, status, source, assigned, linked, tags, dates, sort, group,
  - bulk toolbar: link to case, create case, bulk status/tag/close/delete,
  - карточки алертов:
    - checkbox,
    - delete-кнопка в правом верхнем углу при hover,
    - owner/avatar,
    - severity/status/source/tags,
    - assign-to-me (только если unassigned).
  - пагинация сверху и снизу: стрелки + input страницы + размер страницы.
- Модалки:
  - Create Alert,
  - Create Case from selected alerts,
  - Delete confirm (single/bulk).
- Переходы:
  - карточка -> alert detail,
  - owner/avatar -> user profile,
  - linked case badge -> case detail (через действия).

## 4.5 Alert detail

- Назначение: детальный просмотр и точечные действия.
- Блоки:
  - шапка с id, title, severity, back,
  - summary card (описание, source/status/tags, owner, timestamps, linked case),
  - actions card:
    - assign to me,
    - open linked case,
    - bind to selected case,
    - create case from this alert.
- Переходы: back to alerts, open case.

## 4.6 Cases list

- Назначение: управление кейсами, группировка и bulk-операции.
- Блоки:
  - расширенные фильтры:
    - include/exclude search, mode/logic,
    - severity/status/assigned/assignee,
    - date range, tags,
    - grouping, sorting,
    - include closed,
    - видимость полей карточки.
  - bulk operations: status/close/delete selected.
  - карточки кейсов:
    - checkbox,
    - delete в правом верхнем углу по hover,
    - id/status/tags/title,
    - assignee avatar/name (клик на профиль) или assign-to-me,
    - created/work time,
    - active discussion link (если есть forumId),
    - блок прогресса задач/кастомных полей/responder statuses,
    - severity badge справа.
  - пагинация сверху и снизу + page input + page size.
- Модалки:
  - create case (в т.ч. через шаблон + custom fields),
  - create task from list,
  - delete confirm.
- Переходы:
  - карточка -> case detail,
  - discussion -> forum thread,
  - assignee -> user profile.

## 4.7 Case detail workspace

- Назначение: центральный экран расследования.
- Шапка:
  - editable title (inline),
  - status/severity/priority/stage/TLP badges (часть редактируется inline),
  - AI analyze,
  - share case across tenant,
  - escalate case,
  - copy case,
  - full edit dialog (редактирование всех полей, кроме ID).

- Вкладки:
  1. `Overview`
     - summary stats (tasks/observables/comments),
     - related cases (active recent + all-time, configurable linkBy),
     - closure approvals,
     - case details (inline edit полей),
     - custom fields block,
     - forum widget (start/open discussion),
     - description block с переключателем `Rendered / Raw`.
  2. `Timeline`
     - timeline metrics + status/action breakdown,
     - SLA block + reminders,
     - recent updates,
     - add timeline note,
     - экспорт кейса: JSON/CSV/MD,
     - запуск playbook/workflow из кейса + run status.
  3. `Tasks`
     - create task form,
     - task chain mode (manual/semi/automated),
     - progress + filters,
     - task cards с assignee/status/due/mandatory/delete.
  4. `Observables`
     - add observable (type/value/verdict/tags),
     - table observables,
     - quick add/remove tags, delete observable.
  5. `Visuals`
     - фильтры (source/type/files/search),
     - charts by source/type,
     - applets-таблицы (sources/types/observables/files) с кликабельной фильтрацией.
  6. `Attachments`
     - upload,
     - list/table with type, size, createdAt,
     - preview/open/download.
  7. `Pages`
     - создание длинных wiki-страниц кейса,
     - список страниц с автором и timestamp.
  8. `Comments`
     - поток комментариев,
     - composer нового комментария.
  9. `MITRE`
     - выбор тактик/техник,
     - карточки добавленных mapping’ов,
     - удаление тактик/техник.
  10. `Communications`
      - коммуникации кейса через коннекторы (в т.ч. Telegram/Time),
      - threads list, start thread, message composer,
      - шаблоны сообщений + sync.

## 4.8 DataSec Incidents (`/incidents`)

- Назначение: отдельный DataSec workspace.
- Блоки:
  - KPI по инцидентам,
  - форма регистрации инцидента,
  - список инцидентов с пагинацией и фильтрами,
  - действия по инциденту: ping, escalate, apply measure, close,
  - FAQ panel.

## 4.9 AntiFraud (`/antifraud`)

- Вкладки:
  - `Events`: CRUD событий, фильтры, пагинация.
  - `Incidents`: список инцидентов, маршрут/статус, incident actions (коммуникация/удаление файлов и т.д.), пагинация.
  - `Rules`: CRUD правил маршрутизации и условий.
- Дополнительно: выбор коммуникационных коннекторов для incident action.

## 4.10 Forum list (`/forum`)

- Назначение: список обсуждений по кейсам.
- Блоки:
  - фильтры severity/status,
  - карточки тредов с контекстом linked case, статусом и last activity.
- Переходы:
  - thread card -> forum thread,
  - linked case -> case detail.

## 4.11 Forum thread (`/forum/:id`)

- Назначение: диалог расследования в формате чата.
- Блоки:
  - header + back,
  - case context card (severity, tags, owner, created, open case),
  - message timeline (left/right bubbles, avatars, attachments preview),
  - composer снизу + file attach + send.

## 4.12 Templates (`/templates`)

- Вкладки:
  - `Response Templates`: analyzer/responder template content (Jinja2/HTML).
  - `Communication Templates`: канал/subject/body шаблоны.
  - `Case Templates`: дефолтные параметры кейса + default custom fields.
  - `Observable Types`: типы observable и валидация regex.

## 4.13 Workflow Studio (`/workflow-studio`)

- Двухуровневый сценарий:
  1. `List mode`:
     - список workflow друг под другом,
     - поиск по названию,
     - page size `10/30/50`,
     - пагинация,
     - статус `draft/published` и `enabled/disabled`,
     - CTA: `Create workflow`, `AI Agent`.
  2. `Editor mode`:
     - header: `To workflows`, `New`, `Save`, `Test run`, `Delete`,
     - workflow settings popover (name/description/published/enabled),
     - block library popover (добавление/drag),
     - canvas: pan + drag nodes + connect edges drag-and-drop,
     - inspector справа только при выборе node,
     - run history (последние до 1000 запусков).
- AI Assistant (Sheet справа):
  - чат промптов для генерации workflow JSON,
  - apply suggestion to canvas.
- Node categories в текущем визуале:
  - `Start`, `Control`, `Data`, `Compute`, `Security`, `Action`.

## 4.14 Administration (`/administration`)

- Вкладки:
  1. `User Management`: create/edit/delete users, роли, поиск/списки.
  2. `Tenant Management`: create/edit tenant, activate/deactivate, case statuses panel.
  3. `Achievements`: создание достижений, выбор/загрузка иконок, выдача/отзыв.
  4. `Experience`: ручное начисление XP, история с lazy loading.
  5. `Notification Bots`: tenant-scoped Telegram bots + recipient settings.
  6. `Service Alerts`: правила алертов надежности (low RPS/latency/etc), мгновенные переключатели enabled.
  7. `API Tokens`: create/revoke токены, scopes, expiry.
  8. `Load Management`: tenant resources, module metrics charts, stop/start tenant, rate limits, SOC access policy.
  9. `Connectors`: inbound/outbound connectors, methods, automations.

## 4.15 Profile (`/profile`) и User Profile (`/users/:id`)

- Блоки:
  - cover/avatar, editable profile fields (для своего профиля),
  - achievements grid с rarity,
  - level/xp progress,
  - user performance KPIs,
  - experience history с бескнопочной подгрузкой при скролле.

## 4.16 Security (`/security`)

- Вкладки:
  - `Password`: смена пароля + terminate other sessions.
  - `Sessions`: текущая и другие сессии, revoke.
  - `Notifications`: персональный канал доставки (`in_app`, `telegram`, `email`, `time`) + tenant-scoped bot selection.


