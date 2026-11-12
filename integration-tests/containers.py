"""Podman container management for the integration test suite.

Inspired by pytest-client-tools (candlepin/pytest-client-tools): the test
session owns the container lifecycle through context managers instead of
shell scripts started out-of-band, readiness is validated by response
content rather than "any HTTP 200 answer", and connection details are
passed to tests as values instead of global state.

Unlike pytest-client-tools, the containers are not pinned to a single
well-known host port: each publishes onto the first free port of a
candidate list, so a squatter (cockpit-tls on 9090 being the canonical
example on RHEL cloud images) only forces the next candidate.

The candidates are constrained by SELinux: the daemon runs as
hostmetering_t, whose policy allows outbound connections only to a few
port types (see contrib/selinux/host-metering.te), and both the
subscription-manager helpers it spawns and its own remote write must
reach these test servers. A port from the kernel's ephemeral range
(32768-60999, ephemeral_port_t) would be denied, so the candidates all
come from the allowed types:

- websm_port_t (9090) and the unprivileged http_port_t members
  (9000, 8008, 8009) for the Prometheus receiver
- the unprivileged http_port_t members (8443, 9000, 8008, 8009) for
  Candlepin
"""

import contextlib
import subprocess
import time

import requests

# Timeouts are generous: first-time runs pull the images, and the
# Candlepin webapp takes a while to deploy even with the image cached.
CONTAINER_START_TIMEOUT_S = 300
CANDLEPIN_READY_TIMEOUT_S = 300
PROMETHEUS_READY_TIMEOUT_S = 60

# Candidate host ports, tried in order until one binds. See the module
# docstring for why they must belong to the SELinux-allowed port types.
# 9090 comes first for the receiver because it is the canonical
# Prometheus port; on cockpit guests the bind fails and the next
# http_port_t candidate is used. The two lists share their http_port_t
# tail: the services start at different times, and a candidate taken by
# the other container only forces the next one.
PROMETHEUS_CANDIDATE_PORTS = (9090, 9000, 8008, 8009)
CANDLEPIN_CANDIDATE_PORTS = (8443, 9000, 8008, 8009)

CANDLEPIN_IMAGE = "ghcr.io/candlepin/candlepin-app:latest"
CANDLEPIN_DB_IMAGE = "ghcr.io/candlepin/candlepin-db:latest"
PROMETHEUS_IMAGE = "quay.io/prometheus/prometheus:v3"
CANDLEPIN_NETWORK = "host-metering-test"


def _podman(*args, check=True):
    return subprocess.run(
        ["podman", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=check,
    )


class Container:
    """A podman container running between start() and stop().

    The container is removed on stop (--rm); a failing start (for
    example because a candidate port was taken in the race window)
    raises subprocess.CalledProcessError so the caller can retry with
    the next candidate port. Also usable as a context manager.
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

    def start(self):
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

    def stop(self):
        if self.container_id:
            # Best effort: a stopped container is removed by --rm anyway.
            _podman("stop", self.container_id, check=False)
            self.container_id = None

    def __enter__(self):
        return self.start()

    def __exit__(self, *exc_info):
        self.stop()
        return False


def _remove_stale(names):
    """Best-effort removal of containers a previous, crashed session may
    have left behind. Without this, a leaked container of the same name
    makes every candidate fail with "name already in use", and a
    container still attached to the test network keeps that network
    from being removed and recreated."""
    for name in names:
        _podman("rm", "-f", name, check=False)


@contextlib.contextmanager
def run_container(image, name, publishes, **kwargs):
    """Run a container on the first host port of the candidate list that
    binds. publishes is an iterable of port mappings (see Container); a
    candidate whose port is taken - by a squatter or by the bind race -
    raises subprocess.CalledProcessError and the next one is tried.

    Only the start is guarded: an exception raised in the caller's
    with-body must propagate, not be mistaken for a lost port race.
    """
    last_error = None
    for publish in publishes:
        container = Container(image, name, publish=publish, **kwargs)
        try:
            container.start()
        except subprocess.CalledProcessError as e:
            last_error = e
            continue
        try:
            yield container
        finally:
            container.stop()
        return
    ports = [host_port for publish in publishes for host_port, _ in publish]
    raise RuntimeError(
        f"could not start container {name} on any candidate host port "
        f"{ports}: {last_error.stderr or last_error}"
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

    _remove_stale(("host-metering-test-prometheus",))
    publishes = [((port, 9090),) for port in PROMETHEUS_CANDIDATE_PORTS]
    with run_container(
        PROMETHEUS_IMAGE,
        "host-metering-test-prometheus",
        publishes,
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
    _remove_stale(("host-metering-test-candlepin", "postgres"))
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
            publishes = [
                ((port, 8443),) for port in CANDLEPIN_CANDIDATE_PORTS
            ]
            with run_container(
                CANDLEPIN_IMAGE,
                "host-metering-test-candlepin",
                publishes,
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
