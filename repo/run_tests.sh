#!/usr/bin/env bash
# run_tests.sh — Run tests inside Docker (no local Go install required).
# The app stack must be running first: see README.md
#
# Usage: ./run_tests.sh [suite]
#   suite: all (default), repo, service, handler, jobs, integration, smoke
set -euo pipefail

SUITE=${1:-all}
REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"

TEST_RUN=(
  docker run --rm
  --network docker_default
  -v "${REPO_ROOT}:/src" -w /src
  -e DATABASE_URL=postgres://fulfillops:fulfillops_dev@db:5432/fulfillops?sslmode=disable
  -e FULFILLOPS_SESSION_SECRET=testsessionsecretchars32bytes000
  golang:1.23-alpine
)

case "$SUITE" in
  all)
    "${TEST_RUN[@]}" go test ./... -v -timeout 120s
    ;;
  repo)
    "${TEST_RUN[@]}" go test ./internal/repository/... -v -timeout 60s
    ;;
  service)
    "${TEST_RUN[@]}" go test ./internal/service/... -v -timeout 60s
    ;;
  handler)
    "${TEST_RUN[@]}" go test ./internal/handler/... -v -timeout 60s
    ;;
  jobs)
    "${TEST_RUN[@]}" go test ./internal/job/... -v -timeout 60s
    ;;
  integration)
    "${TEST_RUN[@]}" go test ./tests/integration/... -v -timeout 180s
    ;;
  smoke)
    echo "Tearing down and removing volumes..."
    docker compose -f docker/docker-compose.yml down -v
    echo "Building and starting stack..."
    docker compose -f docker/docker-compose.yml up --build -d
    echo "Waiting for app to be healthy..."
    for i in $(seq 1 30); do
      if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
        echo "App is healthy."
        break
      fi
      sleep 2
    done
    "${TEST_RUN[@]}" go test ./tests/integration/... -v -timeout 180s
    docker compose -f docker/docker-compose.yml down
    ;;
  *)
    echo "Unknown suite: $SUITE"
    echo "Usage: $0 [all|repo|service|handler|jobs|integration|smoke]"
    exit 1
    ;;
esac
