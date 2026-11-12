import contextlib
import datetime
import json
import os
import shutil
import signal
import subprocess
import time
import uuid

import cryptography.hazmat.primitives.hashes
import pytest
import requests
import urllib3
from cryptography import x509

import containers
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

# Integration tests run against a real subscription-manager registered to a
# Candlepin server, and send metering data to a Prometheus remote-write
# receiver. By default both run as podman containers managed by the test
# session itself (see containers.py): they start on first use, publish
# their ports on dynamically allocated localhost ports (so nothing else
# on the host - e.g. cockpit on 9090 - can conflict with them), and are
# torn down when the session ends. Setting CANDLEPIN_HOST or
# PROMETHEUS_URL points the suite at externally managed servers instead
# (e.g. the docker-compose topology in integration-tests/docker used by
# the local developer lane, or shared testing infrastructure).
CANDLEPIN_USERNAME = os.environ.get("CANDLEPIN_USERNAME", "admin")
CANDLEPIN_PASSWORD = os.environ.get("CANDLEPIN_PASSWORD", "admin")
CANDLEPIN_ORG = os.environ.get("CANDLEPIN_ORG", "donaldduck")

# The Candlepin server certificate does not match the service DNS name, so
# subscription-manager is configured with --server.insecure and the status
# polling below skips TLS verification.
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


def generate_cert_and_key(
    common_name="test-host.host-metering.test", org_name="Milton"
):
    private_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
    )
    subject = issuer = x509.Name(
        [
            x509.NameAttribute(NameOID.COUNTRY_NAME, "US"),
            x509.NameAttribute(NameOID.STATE_OR_PROVINCE_NAME, "North Carolina"),
            x509.NameAttribute(NameOID.LOCALITY_NAME, "Raleigh"),
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, org_name),
            x509.NameAttribute(NameOID.COMMON_NAME, common_name),
        ]
    )

    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(private_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(
            datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=1)
        )
        .not_valid_after(
            datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=365)
        )
        .add_extension(
            x509.SubjectAlternativeName([x509.DNSName(common_name)]),
            critical=False,
        )
        .sign(private_key, cryptography.hazmat.primitives.hashes.SHA256())
    )

    cert_pem = cert.public_bytes(serialization.Encoding.PEM)
    key_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.TraditionalOpenSSL,
        encryption_algorithm=serialization.NoEncryption(),
    )
    return cert_pem, key_pem


def _run_subman(args, check=True):
    return subprocess.run(
        ["subscription-manager"] + args,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=check,
    )


def _parse_key_value_output(output):
    values = {}
    for line in output.splitlines():
        if ":" in line:
            key, _, value = line.partition(":")
            # Mirror the daemon's parsing: keys are lowercased because the
            # output casing varies across subscription-manager versions.
            values[key.strip().lower()] = value.strip()
    return values


def _parse_product_ids(output):
    """Product ID values from `subscription-manager list --installed`,
    in output order, mirroring hostinfo.parseSubManOutputMultiVal."""
    products = []
    for line in output.splitlines():
        key, sep, value = line.partition(":")
        if sep and key.strip().lower() == "product id":
            products.append(value.strip())
    return products


class SubManRegistration:
    """Real subscription-manager registration against the Candlepin
    server. Exposes the registered identity, facts, and syspurpose
    values so tests can derive the labels they expect the daemon to
    send."""

    def __init__(self, host, port):
        self.host = host
        self.port = port
        self.identity = {}
        self.facts = {}
        self.syspurpose = {}
        self.products = []
        self.registered = False

    def register(self):
        _run_subman(
            [
                "config",
                "--server.hostname", self.host,
                "--server.port", str(self.port),
                "--server.prefix", "/candlepin",
                "--server.insecure", "1",
            ]
        )
        _run_subman(
            [
                "register",
                "--username", CANDLEPIN_USERNAME,
                "--password", CANDLEPIN_PASSWORD,
                "--org", CANDLEPIN_ORG,
            ]
        )
        # Give the daemon's usage/support labels real values to report.
        for field, value in (("usage", "Production"), ("service-level", "Premium")):
            _run_subman(["syspurpose", field, "--set", value], check=False)
        self._refresh()

    def unregister(self):
        _run_subman(["unregister"], check=False)
        self._refresh()

    def _refresh(self):
        res = _run_subman(["identity"], check=False)
        self.registered = res.returncode == 0
        self.identity = _parse_key_value_output(res.stdout) if self.registered else {}

        if self.registered:
            res = _run_subman(["facts"], check=False)
            self.facts = _parse_key_value_output(res.stdout)

            res = _run_subman(["list", "--installed"], check=False)
            self.products = _parse_product_ids(res.stdout)

            res = _run_subman(["syspurpose", "--show"], check=False)
            if res.returncode == 0:
                try:
                    # Decode only the leading JSON value, mirroring the
                    # daemon: subscription-manager can append warnings
                    # after the payload on stdout.
                    self.syspurpose, _ = json.JSONDecoder().raw_decode(
                        res.stdout.lstrip()
                    )
                except ValueError:
                    self.syspurpose = {}
        else:
            self.facts = {}
            self.syspurpose = {}
            self.products = {}


def _wait_for_candlepin(host, port, timeout_s=180):
    url = f"https://{host}:{port}/candlepin/status"
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        try:
            # While the webapp is still deploying, the app redirects to the
            # Tomcat root page which answers 200. Following the redirect
            # would pass this check too early and make registration fail.
            resp = requests.get(url, verify=False, timeout=5, allow_redirects=False)
            if resp.status_code == 200:
                status = resp.json()
                if "timeUTC" in status and "mode" in status:
                    return
        except (requests.RequestException, ValueError):
            pass
        time.sleep(2)
    raise RuntimeError(f"Candlepin not ready at {url} after {timeout_s}s")


@pytest.fixture(scope="session")
def candlepin_endpoint():
    """The (host, port) of the Candlepin server used by the suite.

    Starts a containerized Candlepin on a dynamically allocated
    localhost port (see containers.candlepin_server) unless
    CANDLEPIN_HOST is set, which points the suite at an externally
    managed server (with CANDLEPIN_PORT, default 8443)."""
    host = os.environ.get("CANDLEPIN_HOST")
    if host:
        yield host, int(os.environ.get("CANDLEPIN_PORT", "8443"))
        return
    with containers.candlepin_server() as (host, port):
        yield host, port


@pytest.fixture(scope="session")
def subman_registration(candlepin_endpoint):
    """Registers the real subscription-manager against the Candlepin
    server once per test session and unregisters on teardown."""
    _wait_for_candlepin(*candlepin_endpoint)
    registration = SubManRegistration(*candlepin_endpoint)
    registration.register()
    yield registration
    registration.unregister()


@pytest.fixture(scope="session")
def host_metering_bin():
    """Returns the path to the host-metering binary. Prioritizes the installed RPM binary if specified."""
    env_bin = os.environ.get("HOST_METERING_BIN")
    if env_bin and os.path.exists(env_bin):
        return env_bin

    bin_path = "/tmp/host-metering"
    src_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    subprocess.run(["go", "build", "-o", bin_path, "."], cwd=src_dir, check=True)
    return bin_path


@pytest.fixture(scope="session")
def prometheus_url():
    """The base URL of the Prometheus remote-write receiver.

    Starts a containerized Prometheus on a dynamically allocated
    localhost port (see containers.prometheus_receiver) unless
    PROMETHEUS_URL is set, which points the suite at an externally
    managed receiver."""
    external = os.environ.get("PROMETHEUS_URL")
    if external:
        yield external
        return
    with containers.prometheus_receiver() as url:
        yield url


# Runtime systemd drop-in directory for the host-metering service: drop-ins
# under /run are transient (tmpfs, cleared on reboot), so even a crashed
# test session cannot leave a permanent service override behind.
UNIT_DROPIN_DIR = "/run/systemd/system/host-metering.service.d"


def _service_can_run():
    """True where the installed host-metering systemd service can run: a
    real systemd init and the RPM installed. The container-based dev
    environment has neither and never starts the service."""
    if not (os.path.exists("/run/systemd/system") and shutil.which("systemctl")):
        return False
    if not shutil.which("rpm"):
        return False
    res = subprocess.run(
        ["rpm", "-q", "host-metering"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    return res.returncode == 0


@pytest.fixture(scope="session", autouse=True)
def service_receiver_write_url(request):
    """Point the installed service's remote write at the session receiver.

    The compiled-in default write_url is the production telemetry
    endpoint (the shipped /etc/host-metering.conf only documents it), so
    every test that starts the stock systemd service would otherwise
    attempt to send the test guest's telemetry there: production rejects
    the test Candlepin's certificates, failing the delivery assertions.

    The pin is a runtime systemd drop-in, named to sort before the proxy
    test's 99-host-metering-test.conf so that test's own write URL wins
    while it runs. It is installed only where the service can run, and
    removed (with the service stopped) when the session ends: left
    running, the service would target the production default again.

    The receiver is resolved lazily, inside the guard, so hosts where
    the service cannot run (e.g. the dev container) never start one for
    this fixture's sake.
    """
    if not _service_can_run():
        yield
        return
    prometheus_url = request.getfixturevalue("prometheus_url")
    dropin = os.path.join(UNIT_DROPIN_DIR, "50-test-receiver.conf")
    try:
        os.makedirs(UNIT_DROPIN_DIR, exist_ok=True)
        with open(dropin, "w") as f:
            f.write(
                "[Service]\n"
                f"Environment=HOST_METERING_WRITE_URL={prometheus_url}/api/v1/write\n"
            )
        subprocess.run(["systemctl", "daemon-reload"], check=True)
        # A service already running when the session starts (e.g. started
        # by the RPM preset on a host where the suite was launched
        # directly) must not keep the production write_url while the
        # early tests run; try-restart re-execs it only if it is active.
        subprocess.run(
            ["systemctl", "try-restart", "host-metering.service"], check=False
        )
        yield prometheus_url
    finally:
        with contextlib.suppress(FileNotFoundError):
            os.unlink(dropin)
        subprocess.run(["systemctl", "daemon-reload"], check=False)
        subprocess.run(
            ["systemctl", "stop", "host-metering.service"], check=False
        )


@pytest.fixture
def cert_generator():
    """Returns a function to generate self-signed certificates on disk."""

    def _generate(
        cert_path,
        key_path,
        common_name="test-host.host-metering.test",
        org_name="Milton",
    ):
        cert_pem, key_pem = generate_cert_and_key(common_name, org_name)
        os.makedirs(os.path.dirname(cert_path), exist_ok=True)
        with open(cert_path, "wb") as f:
            f.write(cert_pem)
        with open(key_path, "wb") as f:
            f.write(key_pem)

    return _generate


@pytest.fixture
def wait_for():
    """Returns a helper that polls a condition until it passes or times out."""

    def _wait_for(condition_fn, timeout_s, interval_s=0.5, desc=""):
        """Poll condition_fn until it returns truthy or timeout_s elapses.

        Exceptions raised by condition_fn (e.g. transient query errors or
        not-yet-satisfied assertions) are retried until the deadline.
        """
        deadline = time.monotonic() + timeout_s
        while True:
            try:
                result = condition_fn()
            except Exception as e:  # transient failures are retried
                print(f"wait_for attempt failed ({desc}): {e}")
                result = None
            if result:
                return result
            if time.monotonic() >= deadline:
                raise AssertionError(
                    f"Timed out after {timeout_s}s waiting for {desc or 'condition'}"
                )
            time.sleep(interval_s)

    return _wait_for


class DaemonProcess:
    def __init__(self, process, stdout_path, stderr_path, stdout_file, stderr_file):
        self.process = process
        self.stdout_path = stdout_path
        self.stderr_path = stderr_path
        self.stdout_file = stdout_file
        self.stderr_file = stderr_file

    def get_stdout(self):
        if not self.stdout_file.closed:
            self.stdout_file.flush()
        with open(self.stdout_path, "r") as f:
            return f.read()

    def get_stderr(self):
        if not self.stderr_file.closed:
            self.stderr_file.flush()
        with open(self.stderr_path, "r") as f:
            return f.read()

    def stop(self, sig=signal.SIGTERM):
        if self.process.poll() is None:
            try:
                os.killpg(os.getpgid(self.process.pid), sig)
                self.process.wait(timeout=5)
            except (OSError, subprocess.SubprocessError):
                try:
                    os.killpg(os.getpgid(self.process.pid), signal.SIGKILL)
                except (OSError, subprocess.SubprocessError):
                    pass
        self.stdout_file.close()
        self.stderr_file.close()


@pytest.fixture
def run_daemon(host_metering_bin):
    """Fixture to run daemon and handle clean shutdown and log collection."""
    processes = []

    def _run(config_path, extra_env=None):
        # Strip HOST_METERING_* variables so the per-test config file is
        # the single source of truth for the daemon's configuration -
        # environment variables would silently override it.
        env = {
            k: v
            for k, v in os.environ.items()
            if not k.startswith("HOST_METERING_")
        }
        if extra_env:
            env.update(extra_env)

        run_id = uuid.uuid4().hex
        stdout_path = f"/tmp/host-metering-{run_id}-stdout.log"
        stderr_path = f"/tmp/host-metering-{run_id}-stderr.log"

        stdout_file = open(stdout_path, "w+")  # noqa: SIM115
        stderr_file = open(stderr_path, "w+")  # noqa: SIM115

        p = subprocess.Popen(
            [host_metering_bin, "-config", config_path, "daemon"],
            env=env,
            stdout=stdout_file,
            stderr=stderr_file,
            start_new_session=True,
        )

        dp = DaemonProcess(p, stdout_path, stderr_path, stdout_file, stderr_file)
        processes.append(dp)

        # Fail fast on a daemon that rejected the configuration and exited
        # before collecting anything, instead of timing out on its logs.
        time.sleep(0.5)
        if p.poll() is not None:
            raise AssertionError(
                f"daemon exited during startup with code {p.returncode}:\n"
                f"{dp.get_stdout()}{dp.get_stderr()}"
            )
        return dp

    yield _run

    for dp in processes:
        try:
            dp.stop()
        except (OSError, subprocess.SubprocessError) as e:
            print(f"Error stopping daemon process: {e}")
        try:
            if os.path.exists(dp.stdout_path):
                os.remove(dp.stdout_path)
            if os.path.exists(dp.stderr_path):
                os.remove(dp.stderr_path)
        except OSError:
            pass
