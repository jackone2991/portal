# syntax=docker/dockerfile:1.7
# Multi-stage build producing two final images: `api` and `worker`.
# Both share the same Go build cache via a common `builder` stage.

# ── Builder ─────────────────────────────────────────────────────
# go 1.25: go.mod requires >= 1.25 since the committed OpenAPI codegen (ADR-10)
# pulls kin-openapi, which needs 1.25. Keep in lockstep with backend/go.mod.
FROM golang:1.25-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# ── API runtime ─────────────────────────────────────────────────
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1000 portal
USER portal
COPY --from=builder /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

# ── Worker runtime (needs ffmpeg + pg_dump/pg_restore) ──────────
# Alpine 3.21 for postgresql17-client — the nightly ops backup (SPEC-09 P0.2)
# pins its client MAJOR to the Postgres 17 server (postgres:17-alpine); a client
# older than the server can refuse to dump. postgresql17-client provides both
# pg_dump (backup task) and pg_restore (restore drill).
FROM alpine:3.21 AS worker
RUN apk add --no-cache ca-certificates tzdata ffmpeg postgresql17-client && adduser -D -u 1000 portal
USER portal
COPY --from=builder /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]
