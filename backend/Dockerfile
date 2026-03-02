FROM golang:1.25-bookworm AS builder
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends gcc g++ pkg-config && rm -rf /var/lib/apt/lists/*

COPY backend/go.mod backend/go.sum ./
COPY backend/third_party ./third_party
RUN go mod download

COPY backend/. ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/incidenthub-api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 ldap-utils && rm -rf /var/lib/apt/lists/*
RUN groupadd --system app && useradd --system --gid app --home-dir /app --no-create-home app
WORKDIR /app
COPY --from=builder /out/incidenthub-api /usr/local/bin/incidenthub-api
COPY --from=builder /app/migrations ./migrations
USER app
EXPOSE 8080
ENTRYPOINT ["incidenthub-api"]
