#!/bin/bash
set -e

BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)

# UBI major used for the containerized integration tests (the devcontainer
# image). Override with --ubi-version or the UBI_VERSION environment
# variable; the authoritative cross-major validation runs on Testing Farm.
DEFAULT_UBI_VERSION=10

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
	if command -v podman-compose &>/dev/null; then
		echo "podman-compose"
	elif command -v docker &>/dev/null && docker compose version &>/dev/null; then
		echo "docker compose"
	elif command -v docker-compose &>/dev/null; then
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
		for _ in {1..30}; do
			if curl -s "http://localhost:${PROMETHEUS_HOST_PORT:-9090}/-/ready" >/dev/null; then
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

	# Build the test container from the devcontainer image
	echo "Building test container (UBI ${version})..."
	UBI_VERSION="${version}" $COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" build tests

	# Run the pytest command inside the container. The write URL is not
	# injected as a HOST_METERING_* environment variable: every test
	# config embeds it from PROMETHEUS_URL, and env vars would silently
	# override per-test config files (defeating e.g. the backlog test's
	# unreachable receiver).
	echo "Running pytest in test container..."
	$COMPOSE_CMD -f "$BASE_DIR/integration-tests/docker/docker-compose.test.yml" run --rm \
		-e PROMETHEUS_URL=http://prometheus:9090 \
		tests pytest "integration-tests/" "${PYTEST_FLAGS[@]}"
}

# Parse arguments
RUN_UNIT=false
RUN_INTEGRATION=false
UBI_VERSION="${UBI_VERSION:-$DEFAULT_UBI_VERSION}"
PYTEST_FLAGS=()

while [[ $# -gt 0 ]]; do
	case "$1" in
	--unit)
		RUN_UNIT=true
		shift
		;;
	--integration)
		RUN_INTEGRATION=true
		shift
		;;
	--ubi-version)
		if [[ -z "$2" || ! "$2" =~ ^(8|9|10)$ ]]; then
			echo "Error: --ubi-version requires 8, 9, or 10." >&2
			exit 1
		fi
		UBI_VERSION="$2"
		RUN_INTEGRATION=true
		shift 2
		;;
	--all)
		RUN_UNIT=true
		RUN_INTEGRATION=true
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
if [ "$RUN_UNIT" = false ] && [ "$RUN_INTEGRATION" = false ]; then
	echo "Usage: $0 [--unit] [--integration] [--ubi-version <8|9|10>] [--all] [pytest_flags...]" >&2
	echo "" >&2
	echo "  --unit            Run Go unit tests on the host" >&2
	echo "  --integration     Run the integration suite in the devcontainer image (default UBI ${DEFAULT_UBI_VERSION})" >&2
	echo "  --ubi-version N   Run the integration suite against UBI N (implies --integration)" >&2
	echo "  --all             Run unit tests and the integration suite" >&2
	exit 1
fi

# Run requested suites
if [ "$RUN_UNIT" = true ]; then
	run_unit_tests
fi

if [ "$RUN_INTEGRATION" = true ]; then
	if run_integration_tests "$UBI_VERSION"; then
		echo "=== Integration tests passed (UBI ${UBI_VERSION}) ==="
	else
		echo "Error: Integration tests failed for UBI ${UBI_VERSION}." >&2
		exit 1
	fi
fi

echo "=== All requested tests completed successfully ==="
