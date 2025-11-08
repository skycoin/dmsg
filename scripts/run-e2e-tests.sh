#!/bin/bash

# E2E Test Runner for DMSG
# This script builds and runs the e2e test environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$SCRIPT_DIR/../docker"
ROOT_DIR="$SCRIPT_DIR/.."

cd "$ROOT_DIR"

echo "==> Building DMSG e2e test environment..."

# Build docker images
cd docker
docker-compose -f docker-compose.e2e.yml build

echo "==> Starting DMSG e2e test services..."
docker-compose -f docker-compose.e2e.yml up -d

echo "==> Waiting for services to be ready..."
sleep 15

echo "==> Running e2e tests..."
cd "$ROOT_DIR"
go test -v -tags !no_ci ./internal/e2e/... || TEST_FAILED=1

echo "==> Cleaning up..."
cd docker
docker-compose -f docker-compose.e2e.yml logs
docker-compose -f docker-compose.e2e.yml down -v

if [ "$TEST_FAILED" == "1" ]; then
    echo "==> Tests FAILED"
    exit 1
fi

echo "==> Tests PASSED"
