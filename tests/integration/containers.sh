#!/bin/bash
# Container lifecycle for the host-metering integration tests on Testing
# Farm guests: a Prometheus remote-write receiver and a local Candlepin
# server for subscription-manager registration.
#
# Uses plain podman - podman-compose is not bundled with podman and would
# add a guest dependency (EPEL on RHEL). The local developer lane uses the
# compose topology in integration-tests/docker/docker-compose.test.yml
# instead; the container names and images here mirror it.
set -euo pipefail

CANDLEPIN_NETWORK=host-metering-test

start_prometheus() {
  cat >/tmp/prometheus-remote-write.yml <<EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s
EOF

  echo "Starting Prometheus receiver..."
  podman rm -f prometheus >/dev/null 2>&1 || true
  podman run -d --name prometheus \
    --net=host \
    -v /tmp/prometheus-remote-write.yml:/etc/prometheus/prometheus.yml:Z \
    quay.io/prometheus/prometheus:v3 \
    --config.file=/etc/prometheus/prometheus.yml \
    --web.enable-remote-write-receiver \
    --web.enable-admin-api

  echo "Waiting for Prometheus to be ready..."
  local ready=false
  for _ in {1..30}; do
    if curl -s --noproxy '*' http://localhost:9090/-/ready >/dev/null; then
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
}

# Start a local Candlepin server (app + database) for the integration
# suite's subscription-manager registration. The database container must
# be named "postgres" - the app resolves it by that name on the shared
# network.
start_candlepin() {
  echo "Starting Candlepin server..."
  # Defensive cleanup in case of a previous (failed) run on this VM.
  podman rm -f candlepin postgres >/dev/null 2>&1 || true
  podman network rm "$CANDLEPIN_NETWORK" >/dev/null 2>&1 || true

  podman network create "$CANDLEPIN_NETWORK" >/dev/null
  podman run -d --name postgres \
    --network "$CANDLEPIN_NETWORK" \
    ghcr.io/candlepin/candlepin-db:latest >/dev/null
  podman run -d --name candlepin \
    --network "$CANDLEPIN_NETWORK" \
    -p 8443:8443 \
    ghcr.io/candlepin/candlepin-app:latest >/dev/null

  echo "Waiting for Candlepin to be ready..."
  local ready=false
  for _ in {1..90}; do
    if curl -sk --noproxy '*' https://localhost:8443/candlepin/status >/dev/null 2>&1; then
      ready=true
      echo "Candlepin is ready."
      break
    fi
    sleep 2
  done
  if [ "$ready" = false ]; then
    echo "Error: Candlepin failed to become ready." >&2
    exit 1
  fi
}

# Idempotent teardown of the test containers: only the local containers
# with the well-known names are removed.
stop() {
  echo "Tearing down container services..."
  podman rm -f prometheus candlepin postgres >/dev/null 2>&1 || true
  podman network rm "$CANDLEPIN_NETWORK" >/dev/null 2>&1 || true
}

case "${1:-}" in
  start-prometheus)
    start_prometheus
    ;;
  start-candlepin)
    start_candlepin
    ;;
  stop)
    stop
    ;;
  *)
    echo "usage: $0 {start-prometheus|start-candlepin|stop}" >&2
    exit 1
    ;;
esac
