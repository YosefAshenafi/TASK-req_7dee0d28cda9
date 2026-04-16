# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates curl && \
    curl -sSfL https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz \
    | tar -xz -C /usr/local/bin migrate

WORKDIR /build

# Download dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Install templ CLI for template generation
RUN GOBIN=/usr/local/bin go install github.com/a-h/templ/cmd/templ@v0.3.819

# Copy source, generate templates, and build
COPY . .
RUN templ generate ./... && \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /fulfillops ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata postgresql-client && \
    addgroup -S fulfillops && \
    adduser -S -G fulfillops fulfillops

WORKDIR /app

COPY --from=builder /fulfillops .
COPY --from=builder /usr/local/bin/migrate /usr/local/bin/migrate
COPY --chown=fulfillops:fulfillops static/ ./static/

# Create runtime directories
RUN mkdir -p /app/exports /app/backups /app/migrations && \
    chown -R fulfillops:fulfillops /app

USER fulfillops

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["./fulfillops"]
