FROM node:22-alpine AS builder
RUN corepack enable
WORKDIR /app

COPY package.json pnpm-workspace.yaml ./
COPY frontend/package.json frontend/package.json
RUN pnpm install --no-frozen-lockfile

COPY frontend ./frontend
ARG VITE_SUPPORT_URL
ARG VITE_UI_SKELETON_MIN_MS
ENV VITE_SUPPORT_URL=${VITE_SUPPORT_URL}
ENV VITE_UI_SKELETON_MIN_MS=${VITE_UI_SKELETON_MIN_MS}
RUN pnpm --filter incidenthub-frontend build

FROM nginx:1.27-alpine
COPY deploy/nginx/frontend.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/frontend/dist /usr/share/nginx/html
EXPOSE 80
ENTRYPOINT ["nginx", "-g", "daemon off;"]
