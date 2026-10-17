# syntax=docker/dockerfile:1

# Build stage with compiler caching
FROM golang:1.25-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Final minimal stage (Kubernetes PodSecurityStandard restricted compliant)
FROM alpine:3.19

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 -S appuser && \
    adduser -u 10001 -S appuser -G appuser

COPY --from=builder --chown=appuser:appuser /app/bin/api /app/api
COPY --from=builder --chown=appuser:appuser /app/migrations /app/migrations

USER appuser

EXPOSE 8080
ENTRYPOINT ["/app/api"]
