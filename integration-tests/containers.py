"""Podman container management for the integration test suite.

Inspired by pytest-client-tools (candlepin/pytest-client-tools): the test
session owns the container lifecycle through context managers instead of
shell scripts started out-of-band, readiness is validated by response
content rather than "any HTTP 200 answer", and connection details are
passed to tests as values instead of global state.

Unlike pytest-client-tools, host ports are allocated dynamically: the
containers publish their fixed container ports onto a free localhost
port, so the suite never conflicts with whatever else listens on the
well-known ports (cockpit-tls on 9090 being the canonical example on
RHEL cloud images).
"""

import contextlib
import socket
import subprocess
import time

import requests

# Timeouts are generous: first-time runs pull the images, and the
# Candlepin webapp takes a while to deploy even with the image cached.
CONTAINER_START_TIMEOUT_S = 300
CANDLEPIN_READY_TIMEOUT_S = 300
PROMETHEUS_READY_TIMEOUT_S = 60

CANDLEPIN_IMAGE = "ghcr.io/candlepin/candlepin-app:latest"
CANDLEPIN_DB_IMAGE = "ghcr.io/candlepin/candlepin-db:latest"
PROMETHEUS_IMAGE = "quay.io/prometheus/prometheus:v3"
CANDLEPIN_NETWORK = "host-metering-test"


def find_free_port():
    """A TCP port currently free on localhost.

    There is an unavoidable race between closing the probe socket and
    the container binding the port; callers pass several candidate ports
    to run_container and the first that binds wins.
    """
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def _podman(*args, check=True):
    return subprocess.run(
        ["podman", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=check,
    )


class Container:
    """A podman container running for the duration of a with-block.

    The container is removed on exit (--rm); a failing start (for
    example because a candidate port was taken in the race window)
    raises subprocess.CalledProcessError so the caller can retry with
    the next candidate port.
    """

    def __init__(self, image, name, args=(), publish=(), network=None,
                 volume=None, pull="newer"):
        self.image = image
        self.name = name
        self.args = list(args)
        # publish: ((host_port, container_port), ...)
        self.publish = list(publish)
        self.network = network
        self.volume = volume
        self.pull = pull
        self.container_id = None

    def __enter__(self):
        argv = ["run", "-d", "--rm", f"--pull={self.pull}", "--name", self.name]
        if self.network:
            argv += ["--network", self.network]
        for host_port, container_port in self.publish:
            argv += ["-p", f"127.0.0.1:{host_port}:{container_port}"]
        if self.volume:
            argv += ["-v", self.volume]
        argv.append(self.image)
        argv += self.args
        res = _podman(*argv)
        self.container_id = res.stdout.strip()
        return self

    def __exit__(self, *exc_info):
        if self.container_id:
            # Best effort: a stopped container is removed by --rm anyway.
            _podman("stop", self.container_id, check=False)
            self.container_id = None
        return False


@contextlib.contextmanager
def run_container(image, name, attempts=3, **kwargs):
    """Run a container, retrying with a fresh port when publishing a
    candidate host port lost the race. publish must be a callable
    returning the port mappings to try."""
    publish_factory = kwargs.pop("publish")
    last_error = None
    for _ in range(attempts):
        publish = publish_factory()
        try:
            with Container(image, name, publish=publish, **kwargs) as c:
                yield c
                return
        except subprocess.CalledProcessError as e:
            last_error = e
    raise RuntimeError(
        f"could not start container {name} after {attempts} attempts"
    ) from last_error


def wait_for_http_json(url, timeout_s, verify=False):
    """Poll an endpoint until it answers with HTTP 200 and a JSON body.

    Unlike a plain curl, this cannot be fooled by an unrelated service
    (cockpit answers 200 HTML on any path) - the body must parse as
    JSON, and callers validate its content.
    """
    deadline = time.monotonic() + timeout_s
    last_error = None
    while time.monotonic() < deadline:
        try:
            resp = requests.get(
                url, verify=verify, timeout=5, allow_redirects=False
            )
            if resp.status_code == 200:
                return resp.json()
            last_error = f"HTTP {resp.status_code}"
        except (requests.RequestException, ValueError) as e:
            last_error = str(e)
        time.sleep(2)
    raise RuntimeError(f"{url} not ready after {timeout_s}s: {last_error}")


@contextlib.contextmanager
def prometheus_receiver():
    """A Prometheus remote-write receiver on a free localhost port.

    Yields the base URL (http://localhost:PORT). The remote-write
    receiver and admin APIs are enabled so tests can both send and
    clear series.
    """
    config = "/tmp/host-metering-test-prometheus.yml"
    with open(config, "w") as f:
        f.write("global:\n  scrape_interval: 15s\n")

    def publish():
        return ((find_free_port(), 9090),)

    with run_container(
        PROMETHEUS_IMAGE,
        "host-metering-test-prometheus",
        publish=publish,
        volume=f"{config}:/etc/prometheus/prometheus.yml:Z",
        args=[
            "--config.file=/etc/prometheus/prometheus.yml",
            "--web.enable-remote-write-receiver",
            "--web.enable-admin-api",
        ],
    ):
        # discover the port that actually bound
        port = _container_port("host-metering-test-prometheus", 9090)
        url = f"http://localhost:{port}"
        status = wait_for_http_json(
            f"{url}/api/v1/status/buildinfo", PROMETHEUS_READY_TIMEOUT_S
        )
        assert status.get("status") == "success", status
        yield url


@contextlib.contextmanager
def candlepin_server():
    """A Candlepin server (app + database) on a free localhost port.

    Yields (host, port). The database container must be named "postgres"
    - the app resolves it by that name on the shared network, mirroring
    the topology of integration-tests/docker/docker-compose.test.yml.
    """
    _podman("network", "rm", CANDLEPIN_NETWORK, check=False)
    _podman("network", "create", CANDLEPIN_NETWORK)

    try:
        # No host port needed for the database: only the app talks to it.
        # The database container must be named "postgres" - the app
        # resolves it by that name on the shared network, mirroring the
        # topology of integration-tests/docker/docker-compose.test.yml.
        with Container(
            CANDLEPIN_DB_IMAGE, "postgres",
            network=CANDLEPIN_NETWORK,
        ):
            def publish():
                return ((find_free_port(), 8443),)

            with run_container(
                CANDLEPIN_IMAGE,
                "host-metering-test-candlepin",
                publish=publish,
                network=CANDLEPIN_NETWORK,
            ):
                port = _container_port("host-metering-test-candlepin", 8443)
                status = wait_for_http_json(
                    f"https://localhost:{port}/candlepin/status",
                    CANDLEPIN_READY_TIMEOUT_S,
                    verify=False,
                )
                # Same content check as conftest._wait_for_candlepin: while
                # the webapp is still deploying, the app redirects to the
                # Tomcat root page.
                assert "timeUTC" in status and "mode" in status, status
                yield "localhost", port
    finally:
        _podman("network", "rm", CANDLEPIN_NETWORK, check=False)


def _container_port(name, container_port):
    """The host port a running container publishes a container port on."""
    res = _podman(
        "port", name, str(container_port)
    )
    # output: "127.0.0.1:32768" (possibly multiple lines, one per family)
    line = res.stdout.strip().splitlines()[0]
    return int(line.rsplit(":", 1)[1])
