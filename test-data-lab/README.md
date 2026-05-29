# IncidentHub Test Data Lab

Отдельный фронтенд для генерации тестовых данных (кейсы, алерты, observables) с AI-помощью.

## Быстрый старт

```bash
cd ./test-data-lab
pnpm install
pnpm dev
```

По умолчанию приложение стартует на `http://localhost:5190`.

## Запуск через Docker Compose

```bash
cd .
docker compose up -d --build test-data-lab
```

- UI будет доступен на `http://localhost:${TEST_DATA_LAB_PORT:-5190}`
- В форму логина автоматически подставляются:
  - `BOOTSTRAP_PLATFORM_ADMIN_EMAIL`
  - `BOOTSTRAP_PLATFORM_ADMIN_PASSWORD`

## Настройки

- `VITE_PROXY_TARGET` — backend для proxy (по умолчанию `http://localhost:8080`)
- `VITE_TEST_DATA_LAB_PORT` — порт dev-сервера (по умолчанию `5190`)

Пример запуска:

```bash
VITE_PROXY_TARGET=http://localhost:8080 VITE_TEST_DATA_LAB_PORT=5190 pnpm dev
```

## Как использовать

1. Заполни `tenant_id` (обязательно).
2. Если нужно — нажми Login (получит token и подставит tenant).
3. Для вкладок **Кейс** / **Алерт** можно нажать `Заполнить AI`, затем `Создать`.
4. Во вкладке **Пакетно** можно массово создать тестовые сущности с AI или локальным генератором.

## Удаление

Весь инструмент изолирован в папке `test-data-lab`.
Чтобы удалить его полностью, просто удали эту папку.
