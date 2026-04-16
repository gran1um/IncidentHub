import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const repoRoot = resolve(__dirname, '../..');
const targetRoot = resolve(__dirname, '../docs-generated');

const docMap = [
  {
    source: 'README.md',
    target: 'overview/project-overview.md',
    title: 'Project Overview',
    description: 'Краткий обзор платформы IncidentHub и состава репозитория.',
  },
  {
    source: 'docs/01-product-functional-spec.md',
    target: 'product/functional-spec.md',
    title: 'Функциональная спецификация',
  },
  {
    source: 'docs/16-platform-functional-inventory.md',
    target: 'product/platform-functional-inventory.md',
    title: 'Каталог функционала платформы',
  },
  {
    source: 'docs/18-designer-screen-spec.md',
    target: 'product/designer-screen-spec.md',
    title: 'Экранное ТЗ для дизайнера',
  },
  {
    source: 'docs/02-frontend.md',
    target: 'architecture/frontend.md',
    title: 'Frontend архитектура',
  },
  {
    source: 'docs/03-backend.md',
    target: 'architecture/backend.md',
    title: 'Backend архитектура',
  },
  {
    source: 'docs/08-connectors.md',
    target: 'architecture/connectors.md',
    title: 'Connectors',
  },
  {
    source: 'docs/09-ai.md',
    target: 'architecture/ai.md',
    title: 'AI подсистема',
  },
  {
    source: 'docs/20-ai-agent-queue-investigation.md',
    target: 'architecture/ai-agent-queue.md',
    title: 'AI Agent Queue и автономное расследование',
  },
  {
    source: 'docs/19-frontend-redesign-implementation-plan.md',
    target: 'architecture/frontend-redesign-plan.md',
    title: 'План переработки frontend',
  },
  {
    source: 'docs/04-api-reference.md',
    target: 'references/api-contracts.md',
    title: 'API карта маршрутов и контрактов',
  },
  {
    source: 'docs/05-postgresql-schema.md',
    target: 'data/postgresql.md',
    title: 'PostgreSQL схема и индексы',
  },
  {
    source: 'docs/06-elasticsearch.md',
    target: 'data/elasticsearch.md',
    title: 'Elasticsearch индексация',
  },
  {
    source: 'docs/07-redis.md',
    target: 'data/redis.md',
    title: 'Redis кеш и rate-limit',
  },
  {
    source: 'docs/10-observability.md',
    target: 'operations/observability.md',
    title: 'Observability',
  },
  {
    source: 'docs/11-infrastructure.md',
    target: 'operations/infrastructure.md',
    title: 'Инфраструктура и деплой',
  },
  {
    source: 'docs/12-security.md',
    target: 'operations/security.md',
    title: 'Безопасность и мультитенантность',
  },
  {
    source: 'docs/13-testing.md',
    target: 'operations/testing.md',
    title: 'Тестирование и качество',
  },
  {
    source: 'docs/14-env-reference.md',
    target: 'operations/env-reference.md',
    title: 'ENV конфигурация',
  },
  {
    source: 'docs/i18n-audit.md',
    target: 'references/i18n-audit.md',
    title: 'i18n аудит фронтенда',
  },
  {
    source: 'backend/tools/loadtest/README.md',
    target: 'appendix/backend-loadtest-tooling.md',
    title: 'Backend Loadtest Tooling',
  },
  {
    source: 'frontend/src/features/workflow-studio/README.md',
    target: 'appendix/frontend-workflow-studio.md',
    title: 'Frontend Workflow Studio',
  },
];

const loadtestReportsDir = resolve(repoRoot, 'backend/loadtest/results');

function ensureDir(path) {
  mkdirSync(path, { recursive: true });
}

function sanitizeMarkdown(input) {
  const lines = input.split('\n');
  let inFence = false;
  const sanitized = lines.map((line) => {
    const trimmed = line.trimStart();
    if (trimmed.startsWith('```')) {
      inFence = !inFence;
      return line;
    }
    if (inFence) {
      return line;
    }
    return line.replace(/</g, '&lt;').replace(/>/g, '&gt;');
  });
  return sanitized.join('\n');
}

function stripLeadingTitle(content) {
  const lines = content.split('\n');
  if (lines.length === 0) {
    return content;
  }
  if (!lines[0].trim().startsWith('# ')) {
    return content;
  }
  const rest = lines.slice(1).join('\n');
  return rest.replace(/^\s*\n/, '');
}

function writeDoc(sourcePath, targetPath, title, description) {
  if (!existsSync(sourcePath)) {
    console.warn(`[docs-web] skipped missing source: ${relative(repoRoot, sourcePath)}`);
    return;
  }

  const sourceRel = relative(repoRoot, sourcePath);
  const rawContent = readFileSync(sourcePath, 'utf8');
  const body = sanitizeMarkdown(stripLeadingTitle(rawContent));

  const frontMatterLines = ['---', `title: ${JSON.stringify(title)}`];
  if (description) {
    frontMatterLines.push(`description: ${JSON.stringify(description)}`);
  }
  frontMatterLines.push('---', '');

  const output = [
    ...frontMatterLines,
    `> Source: \`${sourceRel}\``,
    '',
    body.trim(),
    '',
  ].join('\n');

  ensureDir(dirname(targetPath));
  writeFileSync(targetPath, output, 'utf8');
}

function buildIntroDoc() {
  const introPath = join(targetRoot, 'intro.md');
  const content = `---
title: IncidentHub Docs
description: Единый веб-портал документации по продукту, backend, frontend и API.
---

Добро пожаловать в документацию IncidentHub.

Секция организована по функциональным доменам:

- **Overview**: общий обзор платформы и индекс документации.
- **Product**: продуктовая спецификация и функциональный инвентарь.
- **Architecture**: устройство frontend/backend, AI и connectors.
- **Data Layer**: PostgreSQL, Elasticsearch, Redis.
- **Operations**: инфраструктура, observability, security, testing, env.
- **References**: API контракты и i18n аудит.
- **Appendix**: специализированные техдоки (workflow studio, loadtest tooling).

OpenAPI доступен в отдельном разделе сайта: \`/api\`.
`;
  writeFileSync(introPath, content, 'utf8');
}

function buildDocumentationIndexDoc() {
  const docPath = join(targetRoot, 'overview/documentation-index.md');
  ensureDir(dirname(docPath));
  const content = `---
title: "Documentation Index"
description: "Навигация по ключевым разделам документации IncidentHub."
---

Ниже собраны основные документы с обновленными маршрутами в Docusaurus:

1. [Функциональная спецификация](/guide/product/functional-spec)
2. [Frontend: архитектура и UX](/guide/architecture/frontend)
3. [Backend: архитектура и доменные модули](/guide/architecture/backend)
4. [API: карта маршрутов и контрактов](/guide/references/api-contracts)
5. [PostgreSQL: схема и индексы](/guide/data/postgresql)
6. [Elasticsearch: индексирование и поиск](/guide/data/elasticsearch)
7. [Redis: кеш и rate-limit](/guide/data/redis)
8. [Connectors](/guide/architecture/connectors)
9. [AI подсистема](/guide/architecture/ai)
10. [Observability](/guide/operations/observability)
11. [Инфраструктура и деплой](/guide/operations/infrastructure)
12. [Безопасность](/guide/operations/security)
13. [Тестирование](/guide/operations/testing)
14. [ENV-конфигурация](/guide/operations/env-reference)
15. [Каталог функционала платформы](/guide/product/platform-functional-inventory)
16. [i18n аудит фронтенда](/guide/references/i18n-audit)
17. [Экранное ТЗ для дизайнера](/guide/product/designer-screen-spec)
18. [План переработки frontend](/guide/architecture/frontend-redesign-plan)
19. [AI Agent Queue и автономное расследование](/guide/architecture/ai-agent-queue)

Дополнительные техдоки:

- [Backend Loadtest Tooling](/guide/appendix/backend-loadtest-tooling)
- [Frontend Workflow Studio](/guide/appendix/frontend-workflow-studio)
- [Loadtest Reports](/guide/appendix/loadtest-reports/baseline-20260218)
`;
  writeFileSync(docPath, content, 'utf8');
}

function syncMainDocs() {
  for (const item of docMap) {
    const sourcePath = resolve(repoRoot, item.source);
    const targetPath = join(targetRoot, item.target);
    writeDoc(sourcePath, targetPath, item.title, item.description);
  }
}

function syncLoadtestReports() {
  if (!existsSync(loadtestReportsDir)) {
    return;
  }
  const entries = readdirSync(loadtestReportsDir).filter((entry) => entry.toLowerCase().endsWith('.md')).sort();
  for (const entry of entries) {
    const sourcePath = join(loadtestReportsDir, entry);
    const stats = statSync(sourcePath);
    if (!stats.isFile()) {
      continue;
    }
    const targetPath = join(targetRoot, 'appendix/loadtest-reports', entry);
    const reportTitle = `Loadtest Report: ${entry.replace(/\.md$/i, '')}`;
    writeDoc(sourcePath, targetPath, reportTitle, 'Отчет нагрузки backend/tools/loadtest.');
  }
}

rmSync(targetRoot, { recursive: true, force: true });
ensureDir(targetRoot);
buildIntroDoc();
buildDocumentationIndexDoc();
syncMainDocs();
syncLoadtestReports();

console.log(`[docs-web] synced docs into: ${targetRoot}`);
