FROM node:20-alpine

WORKDIR /app

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json frontend/package.json
COPY docs-web/package.json docs-web/package.json

RUN pnpm install --no-frozen-lockfile --filter incidenthub-docs...

COPY docs-web docs-web
COPY README.md README.md
COPY docs docs
COPY backend backend
COPY frontend frontend

WORKDIR /app/docs-web

EXPOSE 3200

CMD ["pnpm", "run", "start"]
