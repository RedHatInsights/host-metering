#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}/../.." || exit 1

CONTAINERS="$SCRIPT_DIR/containers.sh"

# Cleanup on exit. Idempotent: only local test containers are removed.
trap '"$CONTAINERS" stop' EXIT

# Determine package/binary installation and setup
if [[ -n "${TEST_RPMS:-}" ]]; then
  echo "Installing gating RPMs: ${TEST_RPMS}"
  # shellcheck disable=SC2086  # intentional word splitting of the package list
  dnf -y install --allowerasing ${TEST_RPMS}
elif rpm -q host-metering >/dev/null 2>&1; then
  echo "host-metering is already installed: $(rpm -q host-metering)"
else
  echo "host-metering is not installed. Building and installing locally..."
  make rpm
  dnf install -y --allowerasing dist/rpmbuild/RPMS/*/*.rpm
fi

"$CONTAINERS" start-prometheus

# The integration suite registers the real subscription-manager against a
# local containerized Candlepin (see integration-tests/conftest.py).
"$CONTAINERS" start-candlepin
export CANDLEPIN_HOST=localhost
# Local containers must never be reached through an ambient proxy:
# subscription-manager falls back to HTTP(S)_PROXY environment variables
# when its rhsm.conf proxy fields are blank, and the daemon's Prometheus
# client uses the environment proxy as well.
export NO_PROXY="localhost,127.0.0.1${NO_PROXY:+,$NO_PROXY}"
export no_proxy="$NO_PROXY"
subscription-manager config --server.no_proxy="localhost,127.0.0.1"

# Set up Python virtual environment (outside the repository so the guest
# checkout stays clean)
VENV=/tmp/host-metering-test-venv
python3 -m venv "$VENV"
. "$VENV/bin/activate"
pip install --upgrade pip
pip install -r integration-tests/requirements.txt

# Print versions of relevant RPMs
rpm -q host-metering host-metering-selinux selinux-policy || true

# The Prometheus receiver location, used both as the daemon's write
# target (every test config embeds it) and as the suite's query endpoint.
# The integration tests currently support only a local Prometheus
# receiver; running them against other servers (e.g. stage or prod
# ingest endpoints) is future work. See the cct-cloud-infra project for
# full end-to-end testing against those servers.
export PROMETHEUS_URL=http://localhost:9090
export HOST_METERING_BIN=/usr/bin/host-metering

set +e
pytest --junit-xml=./junit.xml -v integration-tests/
retval=$?
set -e

# Export test results if TMT_PLAN_DATA is defined
if [ -d "${TMT_PLAN_DATA:-}" ] && [ -f ./junit.xml ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
fi

exit $retval
