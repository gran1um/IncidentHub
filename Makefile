GOCACHE ?= $(CURDIR)/backend/.cache/go-build

export GOCACHE

.PHONY: backend-test backend-integration-test backend-run frontend-install frontend-dev frontend-build loadtest password-hash up down

backend-test:
	cd backend && go test ./...

lint:
	cd backend && golangci-lint run ./... -v

backend-integration-test:
	docker compose up -d postgres redis kafka minio
	cd backend && INCIDENTHUB_REQUIRE_DB_TESTS=true go test ./internal/api ./internal/connectors/inbound ./internal/connectors/outbound -run Integration -count=1

backend-run:
	cd backend && go run ./cmd/api

frontend-install:
	pnpm install

frontend-dev:
	pnpm --filter incidenthub-frontend dev

frontend-build:
	pnpm --filter incidenthub-frontend build

loadtest:
	cd backend && go run ./tools/loadtest $(ARGS)

password-hash:
	cd backend && go run ./cmd/password-hash $(ARGS)

up:
	docker compose up -d --build

down:
	docker compose down -v --remove-orphans
