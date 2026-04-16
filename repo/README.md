# FulfillOps — Rewards & Compliance Console

End-to-end reward fulfillment management: tiers, inventory, lifecycle state machine (Draft → Shipped / Voucher Issued → Completed), SLA enforcement, messaging, audit trails, and backup/DR.

**Stack**: Go 1.23 · Gin · Templ · PostgreSQL 16 · Docker

---

## Running with Docker

Docker and Docker Compose are the only requirements.

```bash
# Start the app and PostgreSQL
make up

# App is available at http://localhost:8080
# Stop
make down

# Stop and delete all data volumes (clean slate)
make down-volumes
```

`make up` does everything in one step: generates the encryption key, builds the Go binary, runs database migrations, and starts both containers.

---

## Default Credentials

| Username | Password           | Role          |
|----------|--------------------|---------------|
| `admin`  | `Admin@FulfillOps1`| Administrator |

Change the password after first login in production.

**UI login**: http://localhost:8080/auth/login

**API login**:
```bash
curl -s -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@FulfillOps1"}'
```

---

## Running Tests

Tests run inside Docker — no local Go install needed. Make sure `make up` has been run at least once so the database is available.

```bash
# All test suites
./run_tests.sh

# Individual suites
./run_tests.sh repo         # Repository layer (requires running DB)
./run_tests.sh service      # Service / business logic
./run_tests.sh handler      # HTTP handlers
./run_tests.sh jobs         # Scheduled jobs
./run_tests.sh integration  # End-to-end API tests (requires running DB)

# Full clean build + integration tests + teardown
./run_tests.sh smoke
```

Or use `make` directly:

```bash
make test             # All suites
make test-integration # Integration only
make smoke            # Clean rebuild from scratch, then test
```

---

## Environment Variables

| Variable                          | Default                                                        | Description                     |
|-----------------------------------|----------------------------------------------------------------|---------------------------------|
| `DATABASE_URL`                    | `postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable` | PostgreSQL connection string |
| `FULFILLOPS_ENCRYPTION_KEY_PATH`  | `/app/encryption.key`                                          | Path to 32-byte AES-256 key     |
| `FULFILLOPS_EXPORT_DIR`           | `/app/exports`                                                 | Report CSV output directory     |
| `FULFILLOPS_BACKUP_DIR`           | `/app/backups`                                                 | pg_dump backup directory        |
| `FULFILLOPS_PORT`                 | `8080`                                                         | HTTP listen port                |
| `FULFILLOPS_SESSION_SECRET`       | *(set in docker-compose)*                                      | Cookie session signing key      |

Copy `.env.example` to `.env` and edit before running in production.

---

## Other Useful Commands

```bash
make logs          # Stream app container logs
make db-shell      # Open psql in the database container
make shell         # Open a shell in the app container
make lint          # Run go vet
make migrate-up    # Run pending migrations
make migrate-down  # Roll back the last migration
```
