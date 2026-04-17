# FulfillOps — Rewards & Compliance Console

End-to-end reward fulfillment management: tiers, inventory, lifecycle state machine (Draft → Shipped / Voucher Issued → Completed), SLA enforcement, messaging, audit trails, and backup/DR.

**Stack**: Go 1.23 · Gin · Templ · PostgreSQL 16 · Docker

---

## 1 — Static Review

All business logic is in plain Go under `internal/`. No running stack is required to review the code.

Key areas:

| Area | Path |
|---|---|
| Domain models & state machine | `internal/domain/` |
| SQL migrations (schema + seed) | `migrations/` |
| Service layer (auth, fulfillment, messaging, audit, backup) | `internal/service/` |
| Repository layer (PostgreSQL via pgx/v5) | `internal/repository/` |
| HTTP handlers & router | `internal/handler/` |
| Scheduled jobs | `internal/job/` |
| Templ views | `internal/view/` |
| Unit / handler tests | `internal/*/...` |
| API / integration / E2E tests | `tests/` |

Run unit and handler tests without Docker or a live database:

```bash
go test ./internal/...
```

---

## 2 — Local Run (without Docker)

### Prerequisites

- Go 1.22+
- PostgreSQL 16 (running locally)
- `golang-migrate` CLI — `go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

### Steps

1. **Clone and configure**

   ```bash
   cp .env.example .env
   # Edit .env: set DATABASE_URL, FULFILLOPS_SESSION_SECRET, and the bootstrap-admin vars
   ```

2. **Generate an encryption key**

   ```bash
   make generate-key
   # Copy the generated file path into FULFILLOPS_ENCRYPTION_KEY_PATH in .env
   ```

3. **Run migrations**

   ```bash
   migrate -path ./migrations -database "$DATABASE_URL" up
   ```

4. **Start the server**

   ```bash
   source .env
   go run ./cmd/server/main.go
   ```

   Open **http://localhost:8080/auth/login**

5. **First login**

   Set `FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL` and `FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD` in `.env`
   before starting the server. The bootstrap admin account is created on startup (no-op if an
   administrator already exists). You will be required to change the password on first login.

---

## 3 — Docker Run

Docker is the easiest path for a full local stack.

```bash
docker compose up --build -d
```

Open **http://localhost:8080**

On first start the entrypoint auto-generates a 32-byte AES-256 key at `/app/keystore/encryption.key`
(persisted in the `app_key` Docker volume). Bootstrap credentials are read from the compose
environment — set `FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL` and `FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD` in
your shell or a `.env` file before running.

### Stop

```bash
docker compose down          # keep volumes
docker compose down -v       # also delete all data
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `FULFILLOPS_ENCRYPTION_KEY_PATH` | `/app/keystore/encryption.key` | Path to AES-256 key file (0600, base64) |
| `FULFILLOPS_EXPORT_DIR` | `/app/exports` | Report CSV output directory |
| `FULFILLOPS_BACKUP_DIR` | `/app/backups` | pg_dump backup directory |
| `FULFILLOPS_ASSETS_DIR` | `/app/assets` | Static assets directory |
| `FULFILLOPS_MIGRATIONS_PATH` | `/app/migrations` | SQL migrations directory |
| `FULFILLOPS_PORT` | `8080` | HTTP listen port |
| `FULFILLOPS_SESSION_SECRET` | *(must be set)* | Cookie session signing key (≥32 chars) |
| `FULFILLOPS_SECURE_COOKIES` | `true` | Set `Secure` on session cookies — disable only for local HTTP dev |
| `FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL` | *(empty)* | Email for first-run administrator bootstrap |
| `FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD` | *(empty)* | Password for first-run administrator bootstrap |
| `FULFILLOPS_SCHEDULER_TZ` | *(empty = UTC)* | IANA timezone for daily scheduled jobs |
| `GIN_MODE` | `debug` | Gin framework mode (`debug` / `release` / `test`) |

See `.env.example` for annotated defaults.

---

## Running Tests

Unit and handler tests require no live services:

```bash
go test ./internal/...
```

Integration, API, and E2E tests require a running stack:

```bash
./run_tests.sh              # All suites
./run_tests.sh unit         # Service + handler + repo + job (no DB)
./run_tests.sh api          # Repository + API HTTP tests
./run_tests.sh e2e          # End-to-end + integration + job + config suites
```

---

## Scheduled Jobs

| Name | Cadence | Purpose |
|---|---|---|
| `overdue-check` | every 15 minutes | Open OVERDUE exceptions on SLA breach |
| `notify-retry` | every 10 minutes | Retry failed sends (up to 3×, 30-min window) |
| `backup` | daily at 01:00 | Compliance backup via `pg_dump` (gzipped) |
| `stats` | daily at 02:00 | Refresh cached tier/fulfillment statistics |
| `scheduled-reports` | daily at 02:30 | Generate daily fulfillments + audit exports |
| `cleanup` | daily at 03:00 | Purge soft-deleted rows beyond 30-day recovery window |
| `export-cleanup` | daily at 03:30 | Remove expired report export files |

Daily times are in the timezone set by `FULFILLOPS_SCHEDULER_TZ` (default UTC).

Trigger any job ad-hoc from `/admin/health` or via `POST /api/v1/admin/jobs/:name/run`.

---

## Compliance Controls

- Audit log is append-only at the DB layer (triggers block UPDATE/DELETE) — see `migrations/003_audit_immutability.up.sql`.
- Restore requires integrity verification; the BackupService validates all foreign keys before committing.
- Sensitive exports (PII-unmasked) require the `ADMINISTRATOR` role at create, list, get, verify, and download endpoints.
- Fulfillment transitions enforce optimistic locking (client must supply the current `version`; mismatch → HTTP 409).
- Notification enqueue on a fulfillment transition participates in the same DB transaction.
