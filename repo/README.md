Project type: fullstack

# FulfillOps — Rewards & Compliance Console

End-to-end reward fulfillment management: tiers, inventory, lifecycle state machine (Draft → Shipped / Voucher Issued → Completed), SLA enforcement, messaging, audit trails, and backup/DR.

**Stack**: Go 1.23 · Gin · Templ · PostgreSQL 16 · Docker

---

## 1 — Static Review

All business logic is in plain Go under `internal/`. No running stack is required to review the code.

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

---

## 2 — Startup

Docker is the only required dependency. Run from the `repo/` directory:

```bash
docker-compose up
```

This builds the images and starts PostgreSQL 16 and the application. No `npm install`, `pip install`, `apt-get`, or any other manual setup is needed.

Open **http://localhost:8080/auth/login**

On first start the entrypoint auto-generates a 32-byte AES-256 key at `/app/keystore/encryption.key`
(persisted in the `app_key` Docker volume). The administrator account is created automatically on startup
from the `FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL` / `FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD` env vars in `docker-compose.yml`
— this is a no-op if an administrator already exists.

### Stop

```bash
docker compose down          # keep volumes
docker compose down -v       # also delete all data
```

---

## Demo Credentials

### Auto-seeded on first startup

The administrator account is created automatically from the bootstrap env vars in `docker-compose.yml`. It is ready immediately after `docker-compose up` completes.

| Role          | Username | Email                    | Password            |
|---------------|----------|--------------------------|---------------------|
| Administrator | `admin`  | `admin@fulfillops.local` | `Admin@FulfillOps1` |

### Requires one-time manual seeding

The specialist and auditor accounts do **not** exist until you run the commands below. Run these once after the first `docker-compose up`:

```bash
# 1. Login and save the session cookie
curl -sc /tmp/fo_seed.jar http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@FulfillOps1"}'

# 2. Create the fulfillment specialist account
curl -sb /tmp/fo_seed.jar -X POST http://localhost:8080/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -d '{"username":"specialist","email":"specialist@fulfillops.local","password":"Spec@Demo1!","role":"FULFILLMENT_SPECIALIST"}'

# 3. Create the auditor account
curl -sb /tmp/fo_seed.jar -X POST http://localhost:8080/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -d '{"username":"auditor","email":"auditor@fulfillops.local","password":"Audit@Demo1!","role":"AUDITOR"}'
```

Once seeded, all three accounts are available:

| Role                   | Username     | Email                         | Password      |
|------------------------|--------------|-------------------------------|---------------|
| Administrator          | `admin`      | `admin@fulfillops.local`      | `Admin@FulfillOps1` |
| Fulfillment Specialist | `specialist` | `specialist@fulfillops.local` | `Spec@Demo1!` |
| Auditor                | `auditor`    | `auditor@fulfillops.local`    | `Audit@Demo1!` |

---

## 3 — Verification

After startup (and running the seed commands above), verify the stack is healthy with this curl flow:

### Step 1 — Confirm the app is live

```bash
curl -sf http://localhost:8080/healthz
# → {"status":"ok"}
```

### Step 2 — Confirm the login page renders

```bash
curl -sf http://localhost:8080/auth/login | grep -q 'action=' && echo "UI ok"
# → UI ok
```

### Step 3 — Login as admin and capture the session cookie

```bash
curl -sc /tmp/admin.cookie http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Admin@FulfillOps1"}'
# → HTTP 200  {"id":"...","username":"admin","role":"ADMINISTRATOR"}
```

### Step 4 — Create a reward tier

```bash
curl -sb /tmp/admin.cookie -X POST http://localhost:8080/api/v1/tiers \
  -H "Content-Type: application/json" \
  -d '{"name":"Gold","inventory_count":100,"purchase_limit":5,"alert_threshold":10}'
# → HTTP 201  {"id":"<tier-id>","name":"Gold",...}
```

### Step 5 — List tiers and confirm the new record appears

```bash
curl -sb /tmp/admin.cookie http://localhost:8080/api/v1/tiers
# → HTTP 200  {"items":[{"id":"<tier-id>","name":"Gold",...}],"total":1,...}
```

### Step 6 — Verify admin health

```bash
curl -sb /tmp/admin.cookie http://localhost:8080/api/v1/admin/health
# → HTTP 200  {"status":"ok","checks":{"database":"ok","encryption":"ok",...}}
```

**Expected outcomes**: all six curl calls return the documented HTTP status; `/healthz` and `/api/v1/admin/health` both show `"status":"ok"`; the login page check prints `UI ok`.

---

## Environment Variables

All defaults below are sourced from `docker-compose.yml` (at the repo root, used by the documented `docker-compose up`).

| Variable | Default (docker-compose) | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable` | PostgreSQL connection string |
| `FULFILLOPS_ENCRYPTION_KEY_PATH` | `/app/keystore/encryption.key` | Path to AES-256 key file (auto-generated on first start) |
| `FULFILLOPS_EXPORT_DIR` | `/app/exports` | Report CSV output directory |
| `FULFILLOPS_BACKUP_DIR` | `/app/backups` | pg_dump backup directory |
| `FULFILLOPS_PORT` | `8080` | HTTP listen port |
| `FULFILLOPS_SESSION_SECRET` | `dev-session-secret-change-in-prod!!` | Cookie session signing key (≥32 chars; override via `SESSION_SECRET`) |
| `FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL` | `admin@fulfillops.local` | Email for first-run administrator bootstrap |
| `FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD` | `Admin@FulfillOps1` | Password for first-run administrator bootstrap |
| `GIN_MODE` | `debug` | Gin framework mode (`debug` / `release` / `test`) |

---

## Running Tests

All test suites run inside Docker — no local Go, Node, or PostgreSQL required:

```bash
./run_tests.sh              # All suites (unit + API + E2E)
./run_tests.sh unit         # Domain + util + service tests only
./run_tests.sh api          # Repository + API HTTP tests only
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

Daily times are in UTC. Trigger any job ad-hoc from `/admin/health` or via `POST /api/v1/admin/jobs/:name/run`.

---

## Compliance Controls

- Audit log is append-only at the DB layer (triggers block UPDATE/DELETE) — see `migrations/003_audit_immutability.up.sql`.
- Restore requires integrity verification; the BackupService validates all foreign keys before committing.
- Sensitive exports (PII-unmasked) require the `ADMINISTRATOR` role at create, list, get, verify, and download endpoints.
- Fulfillment transitions enforce optimistic locking (client must supply the current `version`; mismatch → HTTP 409).
- Notification enqueue on a fulfillment transition participates in the same DB transaction.
