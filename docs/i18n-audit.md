# Frontend i18n Audit
Audit date: 2026-02-16

## Scope
- Active app routes and shared layout:
  - `./frontend/src/components/layout.tsx`
  - `./frontend/src/pages/*.tsx`
- Method:
  - Code review of user-facing labels/buttons/messages
  - Regex check for raw English JSX text nodes

## Implemented now
- Added/expanded translation keys in:
  - `./frontend/src/lib/i18n.ts`
- Replaced hardcoded UI strings with `t(...)` in:
  - `./frontend/src/components/layout.tsx`
  - `./frontend/src/pages/login.tsx`
  - `./frontend/src/pages/alerts.tsx`
  - `./frontend/src/pages/cases.tsx`
  - `./frontend/src/pages/forum.tsx`
  - `./frontend/src/pages/forum-thread.tsx`
  - `./frontend/src/pages/profile.tsx`
  - `./frontend/src/pages/security.tsx`
  - `./frontend/src/pages/not-found.tsx`

## Current status by file (raw English JSX text nodes)
- `197` → `./frontend/src/pages/administration.tsx`
- `33` → `./frontend/src/pages/case-detail.tsx`
- `28` → `./frontend/src/pages/templates.tsx`
- `8` → `./frontend/src/pages/cases.tsx`
- `4` → `./frontend/src/pages/profile.tsx`
- `1` → `./frontend/src/pages/dashboard.tsx`
- `0` → `./frontend/src/components/layout.tsx`
- `0` → `./frontend/src/pages/alerts.tsx`
- `0` → `./frontend/src/pages/forum.tsx`
- `0` → `./frontend/src/pages/forum-thread.tsx`
- `0` → `./frontend/src/pages/login.tsx`
- `0` → `./frontend/src/pages/not-found.tsx`
- `0` → `./frontend/src/pages/security.tsx`

## Remaining point-fixes (next batch)
1. `./frontend/src/pages/administration.tsx`
2. `./frontend/src/pages/case-detail.tsx`
3. `./frontend/src/pages/templates.tsx`
4. `./frontend/src/pages/dashboard.tsx` (single literal)
