#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}/../.." || exit 1

# Determine package/binary installation and setup
if [[ -n "${TEST_RPMS:-}" ]]; then
  echo "Installing gating RPMs: ${TEST_RPMS}"
  # shellcheck disable=SC2086  # intentional word splitting of the test RPMs
  dnf -y install --allowerasing ${TEST_RPMS}
elif rpm -q host-metering >/dev/null 2>&1; then
  echo "host-metering is already installed: $(rpm -q host-metering)"
else
  echo "host-metering is not installed. Building and installing locally..."
  make rpm
  dnf install -y --allowerasing dist/rpmbuild/RPMS/*/*.rpm
fi

# The test session manages its own containers (see
# integration-tests/containers.py): a Prometheus remote-write receiver
# and a Candlepin server, both published on candidate localhost ports
# from the SELinux-allowed port types, falling back to the next
# candidate when one is squatted (e.g. by cockpit on 9090). They start
# on first use and are torn down when the session ends; no out-of-band
# setup or cleanup is needed here.

# Stop the service possibly started by the RPM presets: it runs with
# the production write_url, and the suite registering the host would
# make it send real telemetry from the test guest. Tests that need the
# service running start it explicitly.
systemctl stop host-metering.service 2>/dev/null || true

# Local containers must never be reached through an ambient proxy:
# subscription-manager falls back to HTTP(S)_PROXY environment variables
# when its rhsm.conf proxy fields are blank, and the daemon falls back to
# the rhsm.conf proxy fields when the environment has none.
export NO_PROXY="localhost,127.0.0.1${NO_PROXY:+,$NO_PROXY}"
export no_proxy="$NO_PROXY"
subscription-manager config --server.no_proxy="localhost,127.0.0.1"
# The daemon takes its proxy from rhsm.conf when the service environment has
# none; pin that to "no proxy" so a proxy preconfigured in the guest image
# cannot intercept the local Prometheus receiver.
subscription-manager config --server.proxy_hostname="" --server.proxy_port=""

# Set up Python virtual environment (outside the repository so the guest
# checkout stays clean)
VENV=/tmp/host-metering-test-venv
python3 -m venv "$VENV"
. "$VENV/bin/activate"
pip install --upgrade pip
pip install -r integration-tests/requirements.txt

# Print versions of relevant RPMs
rpm -q host-metering host-metering-selinux selinux-policy || true

# The Prometheus receiver location is session-managed (dynamic port)
# unless PROMETHEUS_URL is set; unset it so the suite starts its own.
unset PROMETHEUS_URL
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
