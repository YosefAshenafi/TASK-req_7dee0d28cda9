COMPOSE := docker compose -f docker/docker-compose.yml
APP := fulfillops-app

.PHONY: up down build logs ps \
        migrate-up migrate-down migrate-reset \
        generate-key templ-generate \
        test test-repo test-service test-handler test-jobs test-integration test-frontend smoke \
        seed-demo lint shell db-shell

# ── Docker lifecycle ──────────────────────────────────────────────────────────

up: generate-key
	$(COMPOSE) up --build -d
	@echo "FulfillOps is running at http://localhost:8080"

down:
	$(COMPOSE) down

down-volumes:
	$(COMPOSE) down -v

build:
	$(COMPOSE) build

logs:
	$(COMPOSE) logs -f app

ps:
	$(COMPOSE) ps

# ── Database migrations ───────────────────────────────────────────────────────

migrate-up:
	$(COMPOSE) exec app ./fulfillops -migrate up || \
	$(COMPOSE) run --rm -e DATABASE_URL=postgres://fulfillops:$${DB_PASSWORD:-fulfillops_dev}@db:5432/fulfillops?sslmode=disable \
		--entrypoint "" app \
		sh -c 'migrate -path /app/migrations -database "$$DATABASE_URL" up'

migrate-down:
	$(COMPOSE) run --rm -e DATABASE_URL=postgres://fulfillops:$${DB_PASSWORD:-fulfillops_dev}@db:5432/fulfillops?sslmode=disable \
		--entrypoint "" app \
		sh -c 'migrate -path /app/migrations -database "$$DATABASE_URL" down 1'

migrate-reset:
	$(COMPOSE) run --rm -e DATABASE_URL=postgres://fulfillops:$${DB_PASSWORD:-fulfillops_dev}@db:5432/fulfillops?sslmode=disable \
		--entrypoint "" app \
		sh -c 'migrate -path /app/migrations -database "$$DATABASE_URL" drop -f && migrate -path /app/migrations -database "$$DATABASE_URL" up'

# ── Key generation ────────────────────────────────────────────────────────────

generate-key:
	@if [ ! -f docker/encryption.key.dev ]; then \
		echo "Generating development encryption key..."; \
		dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 | head -c 44 > docker/encryption.key.dev; \
		printf '\n' >> docker/encryption.key.dev; \
		chmod 600 docker/encryption.key.dev; \
		echo "Key created at docker/encryption.key.dev"; \
	fi

# ── Templ code generation ─────────────────────────────────────────────────────

TEMPL_BIN := $(shell which templ 2>/dev/null || echo /tmp/gobin/templ)

templ-generate:
	@[ -x "$(TEMPL_BIN)" ] || GOBIN=/tmp/gobin go install github.com/a-h/templ/cmd/templ@v0.3.819
	$(TEMPL_BIN) generate ./...

# ── Testing (run in go:1.23-alpine container with access to db network) ───────

TEST_RUN = docker run --rm \
	--network docker_default \
	-v $(CURDIR):/src -w /src \
	-e DATABASE_URL=postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable \
	-e FULFILLOPS_SESSION_SECRET=testsessionsecretchars32bytes000 \
	golang:1.23-alpine

test: test-frontend
	$(TEST_RUN) go test ./... -v -timeout 120s

test-frontend:
	docker run --rm \
		-v $(CURDIR)/static/js:/src -w /src \
		node:20-alpine \
		sh -c "npm install --silent && npm test -- --reporter=verbose"

test-repo:
	$(TEST_RUN) go test ./internal/repository/... -v -timeout 60s

test-service:
	$(TEST_RUN) go test ./internal/service/... -v -timeout 60s

test-handler:
	$(TEST_RUN) go test ./internal/handler/... -v -timeout 60s

test-jobs:
	$(TEST_RUN) go test ./internal/job/... -v -timeout 60s

test-integration:
	$(TEST_RUN) go test ./integration/... -v -timeout 180s

seed-demo:
	@echo "Seeding demo specialist and auditor accounts..."
	@COOKIE=$$(curl -s -c /tmp/fo_seed.jar http://localhost:8080/api/v1/auth/login \
		-H "Content-Type: application/json" \
		-d '{"username":"admin","password":"Admin@FulfillOps1"}' | \
		python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('username') else 1)" && \
		grep fulfillops_session /tmp/fo_seed.jar | awk '{print $$NF}') && \
	curl -s -b "fulfillops_session=$$COOKIE" -X POST http://localhost:8080/api/v1/admin/users \
		-H "Content-Type: application/json" \
		-d '{"username":"specialist","email":"specialist@fulfillops.local","password":"Spec@Demo1!","role":"FULFILLMENT_SPECIALIST"}' && \
	curl -s -b "fulfillops_session=$$COOKIE" -X POST http://localhost:8080/api/v1/admin/users \
		-H "Content-Type: application/json" \
		-d '{"username":"auditor","email":"auditor@fulfillops.local","password":"Audit@Demo1!","role":"AUDITOR"}' && \
	echo "Demo users created." || echo "seed-demo: failed (is the stack running?)"

smoke: down-volumes
	$(COMPOSE) up --build -d
	@echo "Waiting for app to be healthy..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then break; fi; \
		sleep 2; \
	done
	$(MAKE) test-integration
	$(MAKE) down

# ── Code quality ──────────────────────────────────────────────────────────────

lint:
	$(COMPOSE) run --rm --entrypoint "" app \
		sh -c 'go vet ./... && echo "vet: OK"'

# ── Shells ────────────────────────────────────────────────────────────────────

shell:
	$(COMPOSE) exec app sh

db-shell:
	$(COMPOSE) exec db psql -U fulfillops -d fulfillops

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo "FulfillOps development commands:"
	@echo "  make up              Build and start all services"
	@echo "  make down            Stop all services"
	@echo "  make down-volumes    Stop and remove all volumes"
	@echo "  make build           Rebuild images"
	@echo "  make logs            Tail app logs"
	@echo "  make migrate-up      Run pending migrations"
	@echo "  make migrate-down    Rollback last migration"
	@echo "  make generate-key    Create dev encryption key (if missing)"
	@echo "  make templ-generate  Compile Templ templates"
	@echo "  make test            Run all tests"
	@echo "  make smoke           Full clean build + integration tests"
	@echo "  make lint            Run go vet"
	@echo "  make shell           Open shell in app container"
	@echo "  make db-shell        Open psql in db container"
