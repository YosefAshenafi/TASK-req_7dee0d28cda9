# FulfillOps — Rewards & Compliance Console

End-to-end reward fulfillment management: tiers, inventory, lifecycle state machine (Draft → Shipped / Voucher Issued → Completed), SLA enforcement, messaging, audit trails, and backup/DR.

**Stack**: Go 1.23 · Gin · Templ · PostgreSQL 16 · Docker

---

## Running the App

Docker is the only requirement.

```bash
docker compose up --build -d
```

Then open **http://localhost:8080**

The app auto-generates its encryption key on first start. No other setup needed.

### Stop

```bash
docker compose down
```

To also delete all data:

```bash
docker compose down -v
```

---

## Default Credentials

| Username | Password            | Role          |
|----------|---------------------|---------------|
| `admin`  | `Admin@FulfillOps1` | Administrator |

**Login page**: http://localhost:8080/auth/login

**API login**:
```bash
curl -s -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@FulfillOps1"}'
```

---

## Running Tests

The stack must be running before running tests (except `smoke`, which manages its own lifecycle).

```bash
./run_tests.sh              # All suites
./run_tests.sh repo         # Repository layer
./run_tests.sh service      # Business logic
./run_tests.sh handler      # HTTP handlers
./run_tests.sh jobs         # Scheduled jobs
./run_tests.sh integration  # End-to-end API tests
./run_tests.sh smoke        # Clean rebuild from scratch + integration tests
```

---

## Environment Variables

| Variable                         | Default                                                                   | Description                    |
|----------------------------------|---------------------------------------------------------------------------|--------------------------------|
| `DATABASE_URL`                   | `postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable` | PostgreSQL connection string   |
| `FULFILLOPS_ENCRYPTION_KEY_PATH` | `/app/keystore/encryption.key`                                            | Path to AES-256 key file       |
| `FULFILLOPS_EXPORT_DIR`          | `/app/exports`                                                            | Report CSV output directory    |
| `FULFILLOPS_BACKUP_DIR`          | `/app/backups`                                                            | pg_dump backup directory       |
| `FULFILLOPS_PORT`                | `8080`                                                                    | HTTP listen port               |
| `FULFILLOPS_SESSION_SECRET`      | *(set in docker-compose)*                                                 | Cookie session signing key     |

Copy `.env.example` to `.env` to override defaults.
