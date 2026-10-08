#!/bin/bash
set -ux

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}/../.." || exit 1

# Read information about release from standard release file
if [[ -f "/etc/os-release" ]]; then
  source /etc/os-release
fi

# Cleanup on exit
cleanup() {
  echo "Tearing down container services..."
  podman rm -f prometheus >/dev/null 2>&1 || true
}
trap 'cleanup' EXIT

# Determine package/binary installation and setup
if [[ -v TEST_RPMS ]]; then
  echo "Installing gating RPMs: ${TEST_RPMS}"
  dnf -y install --allowerasing ${TEST_RPMS}
elif rpm -q host-metering >/dev/null 2>&1; then
  echo "host-metering is already installed: $(rpm -q host-metering)"
else
  echo "host-metering is not installed. Building and installing locally..."
  make rpm
  dnf install -y --allowerasing dist/rpmbuild/RPMS/*/*.rpm
fi

# Write minimal Prometheus configuration
cat <<EOF > /tmp/prometheus-remote-write.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
EOF

# Start Prometheus with remote write receiver enabled
echo "Starting Prometheus receiver..."
podman run -d --name prometheus \
  --net=host \
  -v /tmp/prometheus-remote-write.yml:/etc/prometheus/prometheus.yml:Z \
  quay.io/prometheus/prometheus:latest \
  --config.file=/etc/prometheus/prometheus.yml \
  --web.enable-remote-write-receiver

# Wait for Prometheus to be healthy/ready
echo "Waiting for Prometheus to be ready..."
ready=false
for _ in {1..30}; do
  if curl -s http://localhost:9090/-/ready >/dev/null; then
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

# Set up Python virtual environment
python3 -m venv venv
. venv/bin/activate
pip install --upgrade pip
pip install -r integration-tests/requirements.txt

# Print versions of relevant RPMs
rpm -q host-metering host-metering-selinux selinux-policy || true

# Execute Pytest integration tests
export HOST_METERING_WRITE_URL=http://localhost:9090/api/v1/write
export PROMETHEUS_URL=http://localhost:9090
export HOST_METERING_BIN=/usr/bin/host-metering

pytest --junit-xml=./junit.xml -v integration-tests/
retval=$?

# Export test results if TMT_PLAN_DATA is defined
if [ -d "${TMT_PLAN_DATA:-}" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
fi

exit $retval
