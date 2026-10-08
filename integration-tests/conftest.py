import datetime
import os
import signal
import subprocess
import uuid

import cryptography.hazmat.primitives.hashes
import pytest
from cryptography import x509
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID


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


@pytest.fixture
def mock_env():
    """Sets up environment with mocks in PATH."""
    env = os.environ.copy()
    mocks_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "../mocks"))
    env["PATH"] = f"{mocks_dir}:{env.get('PATH', '')}"
    return env


@pytest.fixture(scope="session")
def prometheus_url():
    """Returns the Prometheus server URL."""
    return os.environ.get("PROMETHEUS_URL", "http://localhost:9090")


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
def run_daemon(host_metering_bin, mock_env):
    """Fixture to run daemon and handle clean shutdown and log collection."""
    processes = []

    def _run(config_path, extra_env=None):
        env = mock_env.copy()
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
