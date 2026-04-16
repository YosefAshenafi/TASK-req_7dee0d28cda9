#!/usr/bin/env bash
# run_tests.sh — runs unit, API, and e2e test suites and prints a results summary.
# Usage:  ./run_tests.sh [--no-docker]
#
# By default uses Docker (same network as the Makefile).
# Pass --no-docker to run directly with DATABASE_URL set in your environment.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COVERAGE_DIR="${REPO_ROOT}/.coverage"
mkdir -p "$COVERAGE_DIR"

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

# ── helpers ───────────────────────────────────────────────────────────────────
pass_count()  { local c; c=$(grep -c "^--- PASS:" "$1" 2>/dev/null) || c=0; echo "$c"; }
fail_count()  { local c; c=$(grep -c "^--- FAIL:" "$1" 2>/dev/null) || c=0; echo "$c"; }
skip_count()  { local c; c=$(grep -c "^--- SKIP:" "$1" 2>/dev/null) || c=0; echo "$c"; }

coverage_pct() {
  local profile="$1"
  if [[ ! -f "$profile" ]]; then echo "N/A"; return; fi
  local pct
  pct=$(go tool cover -func="$profile" 2>/dev/null \
        | awk '/^total:/ {print $3}' | tr -d '%')
  echo "${pct:-N/A}"
}

# ── Docker vs direct ─────────────────────────────────────────────────────────
USE_DOCKER=true
for arg in "$@"; do
  [[ "$arg" == "--no-docker" ]] && USE_DOCKER=false
done

if $USE_DOCKER; then
  if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Use --no-docker or start Docker.${RESET}"
    exit 1
  fi
  RUN_PREFIX="docker run --rm
    --network docker_default
    -v ${REPO_ROOT}:/src -w /src
    -e DATABASE_URL=postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable
    -e FULFILLOPS_SESSION_SECRET=testsessionsecretchars32bytes000
    golang:1.23-alpine"
else
  if [[ -z "${DATABASE_URL:-}" ]]; then
    echo -e "${RED}DATABASE_URL not set. Export it or run without --no-docker.${RESET}"
    exit 1
  fi
  RUN_PREFIX=""
fi

# ── run one suite ─────────────────────────────────────────────────────────────
run_suite() {
  local label="$1"
  local pkg="$2"
  local covpkg="$3"
  local safe="${label// /_}"
  local logfile="${COVERAGE_DIR}/${safe}.log"
  local profile="${COVERAGE_DIR}/${safe}.out"

  echo -e "\n${CYAN}▶  ${BOLD}${label}${RESET}"

  local suite_exit=0
  if $USE_DOCKER; then
    docker run --rm \
      --network docker_default \
      -v "${REPO_ROOT}:/src" -w /src \
      -e DATABASE_URL=postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable \
      -e FULFILLOPS_SESSION_SECRET=testsessionsecretchars32bytes000 \
      golang:1.23-alpine \
      go test -v -count=1 -timeout 180s \
        -coverprofile=".coverage/${safe}.out" \
        -coverpkg="${covpkg}" \
        "${pkg}" > "$logfile" 2>&1 || suite_exit=$?
  else
    go test -v -count=1 -timeout 180s \
      -coverprofile="${profile}" \
      -coverpkg="${covpkg}" \
      "${pkg}" > "$logfile" 2>&1 || suite_exit=$?
  fi

  # Per-test lines
  while IFS= read -r line; do
    if [[ "$line" == "--- PASS:"* ]]; then
      echo -e "  ${GREEN}✓${RESET} ${line#--- PASS: }"
    elif [[ "$line" == "--- FAIL:"* ]]; then
      echo -e "  ${RED}✗${RESET} ${line#--- FAIL: }"
    elif [[ "$line" == "--- SKIP:"* ]]; then
      echo -e "  ${YELLOW}⊘${RESET} ${line#--- SKIP: }"
    fi
  done < "$logfile"

  # If any failure, print the error details
  if [[ "$suite_exit" -ne 0 ]]; then
    echo -e "\n  ${RED}── failure output ──${RESET}"
    grep -A 10 "FAIL\|Error\|panic" "$logfile" | head -40 || true
  fi

  local passed failed skipped
  passed=$(pass_count  "$logfile")
  failed=$(fail_count  "$logfile")
  skipped=$(skip_count "$logfile")
  local total=$(( passed + failed + skipped ))
  local cov
  if $USE_DOCKER; then
    cov=$(grep "coverage:" "$logfile" | tail -1 | awk '{print $2}' | tr -d '%' || echo "N/A")
    [[ -z "$cov" ]] && cov="N/A"
  else
    cov=$(coverage_pct "$profile")
  fi

  printf '%s\t%d\t%d\t%d\t%d\t%s\t%d\n' \
    "$label" "$total" "$passed" "$failed" "$skipped" "$cov" "$suite_exit"
}

# ── Main ─────────────────────────────────────────────────────────────────────
echo -e "${BOLD}"
echo "╔══════════════════════════════════════════════════════════╗"
echo "║         FulfillOps — Test Suite Runner                   ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo -e "${RESET}"

GRAND_TOTAL=0; GRAND_PASS=0; GRAND_FAIL=0; GRAND_SKIP=0
OVERALL_EXIT=0
COV_FAIL=false
COV_THRESHOLD=90
declare -a ROWS=()

# Suite definitions: label | package | coverpkg
declare -a SUITES=(
  "Unit Tests|./tests/unit_tests/|./internal/domain/...,./internal/util/...,./internal/service/..."
  "API Tests|./tests/API_tests/|./internal/handler/...,./internal/repository/...,./internal/service/..."
  "E2E Tests|./tests/e2e_tests/|./internal/..."
)

for SUITE in "${SUITES[@]}"; do
  IFS='|' read -r LABEL PKG COVPKG <<< "$SUITE"
  ROW=$(run_suite "$LABEL" "$PKG" "$COVPKG")
  ROWS+=("$ROW")

  IFS=$'\t' read -r _ total passed failed skipped cov exit_code <<< "$ROW"
  GRAND_TOTAL=$((GRAND_TOTAL + total))
  GRAND_PASS=$((GRAND_PASS   + passed))
  GRAND_FAIL=$((GRAND_FAIL   + failed))
  GRAND_SKIP=$((GRAND_SKIP   + skipped))
  [[ "$exit_code" != "0" ]] && OVERALL_EXIT=1
done

# ── Print summary table ───────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}╔═══════════════════════════════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║                         RESULTS SUMMARY                          ║${RESET}"
echo -e "${BOLD}╠══════════════════╦═══════╦════════╦════════╦═══════╦═════════════╣${RESET}"
printf "${BOLD}║  %-16s║%7s║%8s║%8s║%7s║%13s║${RESET}\n" \
       " Suite" " Tests" " Passed" " Failed" "  Skip" "  Coverage "
echo -e "${BOLD}╠══════════════════╬═══════╬════════╬════════╬═══════╬═════════════╣${RESET}"

for ROW in "${ROWS[@]}"; do
  IFS=$'\t' read -r label total passed failed skipped cov exit_code <<< "$ROW"

  # Colour coverage
  if [[ "$cov" == "N/A" ]]; then
    cov_display="  ${YELLOW}N/A${RESET}      "
    COV_FAIL=true
  else
    cov_int="${cov%%.*}"
    if [[ "$cov_int" -ge "$COV_THRESHOLD" ]]; then
      cov_display="${GREEN}  ${cov}%${RESET}"
    else
      cov_display="${RED}  ${cov}%${RESET}"
      COV_FAIL=true
    fi
  fi

  # Colour failed
  if [[ "$failed" -gt 0 ]]; then
    fail_display="${RED}${failed}${RESET}"
  else
    fail_display="${GREEN}${failed}${RESET}"
  fi

  printf "║  %-16s║%7d║%8d║%7s ║%7d║%-13s║\n" \
    "$label" "$total" "$passed" "${fail_display}" "$skipped" "${cov_display}"
done

echo -e "${BOLD}╠══════════════════╬═══════╬════════╬════════╬═══════╬═════════════╣${RESET}"
printf "${BOLD}║  %-16s║%7d║%8d║%8d║%7d║             ║${RESET}\n" \
       " TOTAL" "$GRAND_TOTAL" "$GRAND_PASS" "$GRAND_FAIL" "$GRAND_SKIP"
echo -e "${BOLD}╚══════════════════╩═══════╩════════╩════════╩═══════╩═════════════╝${RESET}"
echo ""

# ── Verdict ───────────────────────────────────────────────────────────────────
if [[ "$GRAND_FAIL" -gt 0 ]]; then
  echo -e "${RED}${BOLD}✗  FAILED — ${GRAND_FAIL} test(s) did not pass.${RESET}"
  OVERALL_EXIT=1
fi

if $COV_FAIL; then
  echo -e "${RED}${BOLD}✗  COVERAGE below ${COV_THRESHOLD}% threshold in one or more suites.${RESET}"
  echo -e "   Run: go tool cover -html=.coverage/<suite>.out   to inspect gaps."
  OVERALL_EXIT=1
fi

if [[ "$OVERALL_EXIT" -eq 0 ]]; then
  echo -e "${GREEN}${BOLD}✓  ALL ${GRAND_TOTAL} TESTS PASSED — coverage ≥ ${COV_THRESHOLD}% across all suites.${RESET}"
fi

echo ""
exit "$OVERALL_EXIT"
