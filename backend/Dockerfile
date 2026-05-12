# syntax=docker/dockerfile:1.7
# Multi-stage build producing two final images: `api` and `worker`.
# Both share the same Go build cache via a common `builder` stage.

# ── Builder ─────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder
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

# ── Worker runtime (needs ffmpeg) ───────────────────────────────
FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates tzdata ffmpeg && adduser -D -u 1000 portal
USER portal
COPY --from=builder /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]
