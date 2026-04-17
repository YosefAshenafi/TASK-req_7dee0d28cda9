#!/usr/bin/env bash
# run_tests.sh — FulfillOps test runner
# Always runs inside Docker on the compose network, identical to `make test`.
# Starts the stack automatically if it is not already up.
# Streams every test result live; prints a summary table at the end.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${REPO_ROOT}/docker/docker-compose.yml"
COV_DIR="${REPO_ROOT}/.coverage"
mkdir -p "$COV_DIR"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
COV_THRESHOLD=90

DB_URL="postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable"
SESSION_SECRET="testsessionsecretchars32bytes000"

# ── require Docker ────────────────────────────────────────────────────────────
if ! docker info >/dev/null 2>&1; then
  echo -e "${RED}Docker is not running. Please start Docker Desktop and retry.${RESET}"
  exit 1
fi

# ── ensure the compose stack is up ───────────────────────────────────────────
need_stack_start=false
if ! docker network inspect docker_default >/dev/null 2>&1; then
  need_stack_start=true
else
  db_cid="$(docker compose -f "$COMPOSE_FILE" ps -q db 2>/dev/null || true)"
  if [[ -z "$db_cid" ]]; then
    need_stack_start=true
  else
    db_running="$(docker inspect -f '{{.State.Running}}' "$db_cid" 2>/dev/null || echo false)"
    db_health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}unknown{{end}}' "$db_cid" 2>/dev/null || echo unknown)"
    if [[ "$db_running" != "true" || "$db_health" == "unhealthy" ]]; then
      need_stack_start=true
    fi
  fi
fi

if $need_stack_start; then
  echo -e "${CYAN}DB not running or unhealthy — starting db service now...${RESET}"
  docker compose -f "$COMPOSE_FILE" up --build -d --quiet-pull db
fi

echo -ne "${CYAN}Waiting for PostgreSQL to be ready${RESET}"
for i in $(seq 1 30); do
  if docker compose -f "$COMPOSE_FILE" exec -T db \
      pg_isready -U fulfillops -d fulfillops >/dev/null 2>&1; then
    echo -e " ${GREEN}ready${RESET}"
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo -e " ${RED}timeout${RESET}"
    echo -e "${RED}PostgreSQL did not become ready in time.${RESET}"
    exit 1
  fi
  echo -n "."
  sleep 2
done

# ── ensure golang image is cached (no silent pull mid-run) ───────────────────
echo -ne "${CYAN}Ensuring golang:1.23-alpine image is present...${RESET}"
docker pull golang:1.23-alpine --quiet >/dev/null 2>&1 && echo -e " ${GREEN}ok${RESET}" || true

# ── line colorizer (called while piping go test -v output) ───────────────────
colorize() {
  while IFS= read -r line; do
    case "$line" in
      "--- PASS:"*) echo -e "  ${GREEN}✓${RESET}  ${line#--- PASS: }" ;;
      "--- FAIL:"*) echo -e "  ${RED}✗${RESET}  ${line#--- FAIL: }"
                   # Echo next few lines as failure detail
                   ;;
      "--- SKIP:"*) echo -e "  ${YELLOW}⊘${RESET}  ${line#--- SKIP: }" ;;
      "go: downloading"*) echo -e "    ${CYAN}${line}${RESET}" ;;
      "go: finding module"*) echo -e "    ${CYAN}${line}${RESET}" ;;
      "=== RUN"*|"=== PAUSE"*|"=== CONT"*) ;;  # suppress
      FAIL*|ok\ *|coverage:*) echo "    $line" ;;
      *FAIL*|*panic*|*Error*) echo -e "    ${RED}${line}${RESET}" ;;
      *) echo "    $line" ;; # keep startup/build/progress lines visible
    esac
  done
}

# ── run one suite — output streams directly to terminal ──────────────────────
run_suite() {
  local label="$1" pkg="$2" covpkg="$3"
  local safe="${label// /_}"
  local logfile="${COV_DIR}/${safe}.log"
  local resultfile="${COV_DIR}/${safe}.result"

  echo ""
  echo -e "${BOLD}${CYAN}━━━  ${label}  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo "    starting: go test ${pkg}"

  local suite_exit=0

  docker run --rm \
    --network docker_default \
    -v "${REPO_ROOT}:/src" -w /src \
    -v fulfillops_go_mod_cache:/go/pkg/mod \
    -v fulfillops_go_build_cache:/root/.cache/go-build \
    -e "DATABASE_URL=${DB_URL}" \
    -e "FULFILLOPS_SESSION_SECRET=${SESSION_SECRET}" \
    golang:1.23-alpine \
    go test -v -count=1 -timeout 180s \
      -coverprofile=".coverage/${safe}.out" \
      -coverpkg="${covpkg}" \
      ${pkg} 2>&1 | tee "$logfile" | colorize
  suite_exit=${PIPESTATUS[0]}

  # Stats
  local passed failed skipped
  passed=$(grep -c  "^--- PASS:" "$logfile" 2>/dev/null || true); passed=${passed:-0}
  failed=$(grep -c  "^--- FAIL:" "$logfile" 2>/dev/null || true); failed=${failed:-0}
  skipped=$(grep -c "^--- SKIP:" "$logfile" 2>/dev/null || true); skipped=${skipped:-0}
  local total=$(( passed + failed + skipped ))

  # Merged statement % for -coverpkg (go test's per-package "coverage:" line is not the merge total).
  # Prefer a local `go tool cover` — it's instant. Fall back to docker when Go isn't installed.
  local cov="N/A"
  if [[ -f "${COV_DIR}/${safe}.out" ]] && [[ "${suite_exit}" -eq 0 ]]; then
    if command -v go >/dev/null 2>&1; then
      cov=$(cd "${REPO_ROOT}" && go tool cover -func ".coverage/${safe}.out" 2>/dev/null \
        | grep '^total:' | grep -oE '[0-9]+\.[0-9]+' | tail -1)
    else
      cov=$(timeout 60 docker run --rm \
          -v "${REPO_ROOT}:/src" -w /src \
          -v fulfillops_go_mod_cache:/go/pkg/mod \
          -v fulfillops_go_build_cache:/root/.cache/go-build \
          golang:1.23-alpine \
          go tool cover -func ".coverage/${safe}.out" 2>/dev/null \
        | grep '^total:' | grep -oE '[0-9]+\.[0-9]+' | tail -1)
    fi
    cov="${cov:-N/A}"
  fi

  # Persist result for the summary table
  printf '%s\t%d\t%d\t%d\t%d\t%s\t%d\n' \
    "$label" "$total" "$passed" "$failed" "$skipped" "$cov" "$suite_exit" \
    > "$resultfile"
}

# ── header ────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════════════╗"
echo    "║         FulfillOps — Test Suite Runner                   ║"
echo -e "╚══════════════════════════════════════════════════════════╝${RESET}"

declare -a SUITES=(
  "Unit Tests|./tests/unit_tests/ ./internal/service|./internal/domain/...,./internal/util/..."
  "API Tests|./tests/API_tests/ ./internal/repository|./internal/repository/..."
  "E2E Tests|./tests/e2e_tests/ ./internal/job ./internal/config ./internal/middleware|./internal/job/...,./internal/config/...,./internal/middleware/..."
)

# ── run suites (output streams live — NOT inside $()) ────────────────────────
for SUITE in "${SUITES[@]}"; do
  IFS='|' read -r LABEL PKG COVPKG <<< "$SUITE"
  run_suite "$LABEL" "$PKG" "$COVPKG"
done

# ── read results and print summary table ─────────────────────────────────────
GRAND_TOTAL=0; GRAND_PASS=0; GRAND_FAIL=0; GRAND_SKIP=0
OVERALL_EXIT=0; COV_FAIL=false
declare -a ROWS=()

for SUITE in "${SUITES[@]}"; do
  IFS='|' read -r LABEL _ _ <<< "$SUITE"
  safe="${LABEL// /_}"
  ROW=$(cat "${COV_DIR}/${safe}.result" 2>/dev/null \
        || printf '%s\t0\t0\t0\t0\tN/A\t1\n' "$LABEL")
  ROWS+=("$ROW")

  IFS=$'\t' read -r _ total passed failed skipped cov xcode <<< "$ROW"
  GRAND_TOTAL=$((GRAND_TOTAL + total))
  GRAND_PASS=$((GRAND_PASS   + passed))
  GRAND_FAIL=$((GRAND_FAIL   + failed))
  GRAND_SKIP=$((GRAND_SKIP   + skipped))
  [[ "$xcode" != "0" ]] && OVERALL_EXIT=1
done

echo ""
echo -e "${BOLD}╔══════════════════════╦════════╦════════╦════════╦═══════╦══════════╗"
echo    "║  Suite               ║  Tests ║ Passed ║ Failed ║  Skip ║  Cov% ║"
echo -e "╠══════════════════════╬════════╬════════╬════════╬═══════╬══════════╣${RESET}"

for ROW in "${ROWS[@]}"; do
  IFS=$'\t' read -r label total passed failed skipped cov xcode <<< "$ROW"

  # Coverage colour (N/A = no merged profile or failed suite — do not treat as below-threshold)
  if [[ "$cov" == "N/A" ]]; then
    cov_col="${YELLOW}N/A${RESET}    "
  else
    cov_int="${cov%%.*}"
    if (( cov_int >= COV_THRESHOLD )); then
      cov_col="${GREEN}${cov}%${RESET}"
    else
      cov_col="${RED}${cov}%${RESET}"; COV_FAIL=true
    fi
  fi

  # Failed colour
  if (( failed > 0 )); then
    fail_col="${RED}${failed}${RESET}"
  else
    fail_col="${GREEN}${failed}${RESET}"
  fi

  printf "║  %-20s║%8d║%8d║  " "$label" "$total" "$passed"
  printf "%b%-4s  ║%7d║  %-8b║\n" "${fail_col}" "" "$skipped" "${cov_col}"
done

echo -e "${BOLD}╠══════════════════════╬════════╬════════╬════════╬═══════╬══════════╣"
printf    "║  %-20s║%8d║%8d║%8d║%7d║          ║\n" \
          " TOTAL" "$GRAND_TOTAL" "$GRAND_PASS" "$GRAND_FAIL" "$GRAND_SKIP"
echo -e   "╚══════════════════════╩════════╩════════╩════════╩═══════╩══════════╝${RESET}"
echo ""

# ── verdict ───────────────────────────────────────────────────────────────────
(( GRAND_FAIL > 0 )) && {
  echo -e "${RED}${BOLD}✗  ${GRAND_FAIL} test(s) FAILED${RESET}"; OVERALL_EXIT=1
}
$COV_FAIL && {
  echo -e "${RED}${BOLD}✗  Coverage below ${COV_THRESHOLD}% in one or more suites${RESET}"
  echo    "   Inspect: go tool cover -html=${COV_DIR}/<Suite_Name>.out"
  OVERALL_EXIT=1
}
(( OVERALL_EXIT == 0 )) && \
  echo -e "${GREEN}${BOLD}✓  ALL ${GRAND_TOTAL} TESTS PASSED  —  coverage ≥ ${COV_THRESHOLD}% in every suite${RESET}"

echo ""
exit "$OVERALL_EXIT"
