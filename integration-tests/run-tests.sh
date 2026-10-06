#!/bin/bash
set -e

BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)

# Tracking if we spun up Prometheus container
SPUN_UP=false

cleanup() {
  if [ "$SPUN_UP" = "true" ]; then
    echo "Tearing down container services..."
    $COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}

# Exit handler for clean teardown
trap 'cleanup' EXIT

get_compose_cmd() {
  if command -v podman-compose &> /dev/null; then
    echo "podman-compose"
  elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
    echo "docker compose"
  elif command -v docker-compose &> /dev/null; then
    echo "docker-compose"
  else
    echo "No compose tool (podman-compose, docker compose, docker-compose) found!" >&2
    exit 1
  fi
}

COMPOSE_CMD=$(get_compose_cmd)

start_prometheus() {
  if [ "$SPUN_UP" = "false" ]; then
    echo "Starting Prometheus using $COMPOSE_CMD..."
    $COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" up -d prometheus
    SPUN_UP=true
    
    # Wait for Prometheus to be healthy/ready
    echo "Waiting for Prometheus to be ready..."
    local ready=false
    for i in {1..30}; do
      if curl -s http://localhost:9090/-/ready > /dev/null; then
        ready=true
        echo "Prometheus is ready."
        break
      fi
      sleep 1
    done
    if [ "$ready" = false ]; then
      echo "Error: Prometheus failed to become ready." >&2
      exit 1
    fi
  fi
}

run_unit_tests() {
  echo "=== Running Go Unit Tests ==="
  PATH="$BASE_DIR/mocks:$PATH" go test -v ./...
}

run_integration_tests() {
  local version=$1
  echo "=== Running Integration Tests against UBI ${version} ==="
  
  start_prometheus
  
  # Build the specific UBI container service
  echo "Building ubi${version} container..."
  $COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" build "ubi${version}"
  
  # Run the pytest command inside the container
  echo "Running pytest in ubi${version} container..."
  $COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" run --rm \
    -e HOST_METERING_WRITE_URL=http://prometheus:9090/api/v1/write \
    -e PROMETHEUS_URL=http://prometheus:9090 \
    "ubi${version}" pytest "integration-tests/" "${PYTEST_FLAGS[@]}"
}

# Parse arguments
RUN_UNIT=false
UBI_VERSIONS=()
PYTEST_FLAGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --unit)
      RUN_UNIT=true
      shift
      ;;
    --ubi-version)
      if [[ -z "$2" || ! "$2" =~ ^(8|9|10)$ ]]; then
        echo "Error: --ubi-version requires 8, 9, or 10." >&2
        exit 1
      fi
      UBI_VERSIONS+=("$2")
      shift 2
      ;;
    --all)
      RUN_UNIT=true
      UBI_VERSIONS=(8 9 10)
      shift
      ;;
    -*)
      # Pass through any flags starting with '-' to pytest
      PYTEST_FLAGS+=("$1")
      # Check if the next argument is a value for this flag
      if [[ $# -gt 1 && ! "$2" =~ ^- ]]; then
        PYTEST_FLAGS+=("$2")
        shift
      fi
      shift
      ;;
    *)
      # Pass through any non-flag arguments as pytest selectors / tests
      PYTEST_FLAGS+=("$1")
      shift
      ;;
  esac
done

# If no actions specified, show usage
if [ "$RUN_UNIT" = false ] && [ ${#UBI_VERSIONS[@]} -eq 0 ]; then
  echo "Usage: $0 [--unit] [--ubi-version <8|9|10>] [--all] [pytest_flags...]" >&2
  exit 1
fi

# Run requested suites
if [ "$RUN_UNIT" = "true" ]; then
  run_unit_tests
fi

for ver in "${UBI_VERSIONS[@]}"; do
  run_integration_tests "$ver"
done

echo "=== All requested tests completed successfully ==="
