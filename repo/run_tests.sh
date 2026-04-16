#!/usr/bin/env bash
# run_tests.sh — Run all tests inside Docker (no local Go install required).
# Usage: ./run_tests.sh [suite]
#   suite: all (default), repo, service, handler, jobs, integration
set -euo pipefail

SUITE=${1:-all}

case "$SUITE" in
  all)         make test ;;
  repo)        make test-repo ;;
  service)     make test-service ;;
  handler)     make test-handler ;;
  jobs)        make test-jobs ;;
  integration) make test-integration ;;
  smoke)       make smoke ;;
  *)
    echo "Unknown suite: $SUITE"
    echo "Usage: $0 [all|repo|service|handler|jobs|integration|smoke]"
    exit 1
    ;;
esac
