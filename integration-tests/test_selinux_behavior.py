"""SELinux and systemd behavior tests for the installed service.

These verify the behavior that only appears when the service is started
by systemd on a real system with enforcing SELinux (Testing Farm VMs or
bare metal); the container-based dev environment skips them through the
rpm/systemd/selinux marks:

- the service starts via systemd and stays running (no crash loop);
- the daemon process actually transitions into hostmetering_t (the
  transition that is lost when ExecStart points at a wrapper shell
  script instead of the daemon binary);
- file contexts are applied to the binary, the unit file and the
  runtime directory;
- no SELinux denials are triggered by the daemon or its helper
  subprocesses (subscription-manager, dmidecode, ip, ...);
- the proxy configured in /etc/rhsm/rhsm.conf is read by the daemon and the
  remote write is actually sent through that proxy.
"""

import contextlib
import os
import re
import shutil
import socketserver
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest
import requests

SERVICE = "host-metering.service"
DAEMON = "/usr/bin/host-metering"
UNIT_FILE = "/usr/lib/systemd/system/host-metering.service"
RUNTIME_DIR = "/run/host-metering"
AUDIT_LOG = "/var/log/audit/audit.log"
RHSM_CONF = "/etc/rhsm/rhsm.conf"
DAEMON_CONTEXT = "system_u:system_r:hostmetering_t:s0"
# Runtime systemd drop-ins: transient by design, so a failed test cannot
# leave a permanent service override behind.
UNIT_DROPIN_DIR = "/run/systemd/system/host-metering.service.d"
# The daemon's write target during the proxy test. It must not match the
# RHSM no_proxy list that keeps subscription-manager on its direct route to
# Candlepin, and it must not resolve: an HTTP proxy receives the absolute
# URI, so the daemon never looks the name up.
PROXY_TEST_WRITE_URL = "http://write.example.test/api/v1/write"


def _rpm_installed():
    """True when the host-metering RPM is installed (e.g. on Testing Farm)."""
    if not shutil.which("rpm"):
        return False
    res = subprocess.run(
        ["rpm", "-q", "host-metering"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return res.returncode == 0


def _systemd_available():
    """True when systemd is the running init system (VMs, not containers)."""
    return os.path.exists("/run/systemd/system") and shutil.which("systemctl")


def _selinux_module_active():
    """True when SELinux is enforcing and the host-metering policy module
    is loaded. Permissive mode is not enough: denials would be logged but
    allowed, making test_no_avc_denials_for_hostmetering fail confusingly
    instead of skipping."""
    if not shutil.which("getenforce"):
        return False
    res = subprocess.run(
        ["getenforce"], stdout=subprocess.PIPE, universal_newlines=True
    )
    if res.stdout.strip() != "Enforcing":
        return False
    res = subprocess.run(
        ["semodule", "-l"], stdout=subprocess.PIPE, universal_newlines=True
    )
    return any(
        line.startswith("host-metering") for line in res.stdout.splitlines()
    )


def _systemctl(*args, check=True):
    return subprocess.run(
        ["systemctl", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=check,
    )


def _start_service_fresh():
    """(Re)start the service with a clean restart counter."""
    _systemctl("reset-failed", SERVICE, check=False)
    _systemctl("restart", SERVICE)


def _start_service_with_empty_wal():
    """Start the service with a cleared runtime directory.

    RuntimeDirectoryPreserve=yes keeps the metrics WAL across restarts, so the
    service can still hold samples collected before this test ran. The receiver
    orders samples per series, and the suite's other daemons
    (test_backlog_recovery, test_collection) already pushed newer samples for the
    same label set through the same consumer identity, so replaying the older
    backlog is rejected with "out of order sample" (HTTP 400) and the daemon
    drops the batch. A reboot clears /run, so clearing the runtime directory
    here reproduces the state this test asserts on: the certificate read and the
    delivery inside hostmetering_t, not the ordering of a stale backlog.
    """
    _systemctl("stop", SERVICE, check=False)
    shutil.rmtree(RUNTIME_DIR, ignore_errors=True)
    _start_service_fresh()


def _main_pid():
    res = _systemctl("show", "-p", "MainPID", "--value", SERVICE)
    pid = res.stdout.strip()
    return int(pid) if pid.isdigit() else None


def _stat_context(path):
    res = subprocess.run(
        ["stat", "-c", "%C", path],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
    )
    return res.stdout.strip() if res.returncode == 0 else None


def _audit_denials(lines):
    """AVC denial lines involving hostmetering, excluding the normally
    dont-audited process-transition permissions (noatsecure, rlimitinh,
    siginh) that only show up when dontaudit rules are disabled for
    debugging. Other process-class denials (signals, ptrace, ...) are
    real findings and are kept."""
    _transition_noise = ("noatsecure", "rlimitinh", "siginh")
    return [
        line.strip()
        for line in lines
        if "avc:  denied" in line
        and "hostmetering" in line
        and not any(perm in line for perm in _transition_noise)
    ]


def _journal_tail(since_minutes=3):
    res = subprocess.run(
        ["journalctl", "-u", "host-metering", "--no-pager",
         "--since", f"-{since_minutes}min"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        universal_newlines=True,
    )
    return res.stdout


def _journal_for_pid(pid):
    """Journal lines written by an exact daemon process, so a previous
    service instance cannot satisfy (or pollute) the assertions."""
    res = subprocess.run(
        ["journalctl", "-u", SERVICE, f"_PID={pid}", "--no-pager"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        universal_newlines=True,
    )
    return res.stdout


def _audit_offset():
    try:
        with open(AUDIT_LOG) as f:
            return sum(1 for _ in f)
    except OSError:
        return None


def _denials_since(offset):
    if offset is None:
        return []
    with open(AUDIT_LOG) as f:
        return _audit_denials(f.readlines()[offset:])


@contextlib.contextmanager
def _rhsm_proxy(hostname=None, port=None, no_proxy=None, env=None):
    """Temporarily write proxy fields into the [server] section of
    /etc/rhsm/rhsm.conf and optional Environment= entries into a runtime
    systemd drop-in, restart the service so the daemon re-reads both, and
    restore them on exit. Fields passed as None are left untouched; a key
    missing from the guest's template is inserted right after the [server]
    header. Only the [server] section is touched: the same option names
    exist in other sections and must keep their values."""
    shutil.copyfile(RHSM_CONF, RHSM_CONF + ".test-backup")
    dropin = None
    try:
        if env:
            os.makedirs(UNIT_DROPIN_DIR, exist_ok=True)
            dropin = os.path.join(UNIT_DROPIN_DIR, "99-host-metering-test.conf")
            with open(dropin, "w") as f:
                f.write("[Service]\n")
                for name, value in env.items():
                    f.write(f'Environment="{name}={value}"\n')
            _systemctl("daemon-reload")
        with open(RHSM_CONF) as f:
            lines = f.read().splitlines(keepends=True)
        try:
            start = next(
                i for i, line in enumerate(lines) if line.strip() == "[server]"
            )
        except StopIteration:
            raise AssertionError(f"no [server] section in {RHSM_CONF}")
        end = next(
            (
                i
                for i in range(start + 1, len(lines))
                if lines[i].lstrip().startswith("[")
            ),
            len(lines),
        )
        for key, value in (
            ("proxy_hostname", hostname),
            ("proxy_port", port),
            ("no_proxy", no_proxy),
        ):
            if value is None:
                continue
            key_re = re.compile(rf"^\s*{key}\s*=")
            for i in range(start + 1, end):
                if key_re.match(lines[i]):
                    lines[i] = f"{key} = {value}\n"
                    break
            else:
                lines.insert(start + 1, f"{key} = {value}\n")
                end += 1
        with open(RHSM_CONF, "w") as f:
            f.write("".join(lines))
        _start_service_fresh()
        yield
    finally:
        shutil.move(RHSM_CONF + ".test-backup", RHSM_CONF)
        if dropin is not None and os.path.exists(dropin):
            os.unlink(dropin)
            _systemctl("daemon-reload")
        _start_service_fresh()


@pytest.mark.rpm
@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_rhsm_proxy_config_is_loaded():
    """The daemon must read the proxy from /etc/rhsm/rhsm.conf itself: the
    resolved proxy URL is part of the configuration the daemon logs at
    startup, so an empty value means the file (or its SELinux access) is
    not working. No helper script or environment file is involved."""
    with _rhsm_proxy(hostname="127.0.0.1", port="3128"):
        pid = _main_pid()
        assert pid, "the service did not restart"
        journal = _journal_for_pid(pid)
        deadline = time.time() + 30
        while time.time() < deadline and \
                "|  ProxyUrl: http://127.0.0.1:3128" not in journal:
            time.sleep(1)
            journal = _journal_for_pid(pid)
        assert "|  ProxyUrl: http://127.0.0.1:3128" in journal, (
            "the daemon did not load the proxy from rhsm.conf; journal:\n"
            + journal
        )


@pytest.mark.rpm
@pytest.mark.systemd
@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed()
    or not _systemd_available()
    or not _selinux_module_active(),
    reason="requires installed RPM, systemd and enforcing SELinux "
    "(Testing Farm VM)",
)
def test_registered_service_sends_without_denials(
    subman_registration, prometheus_url
):
    """End-to-end on a registered host: the systemd-started daemon must
    read its consumer certificate and key (cert_t files) for the mTLS
    connection, deliver samples to the Prometheus receiver, and do all
    of it inside hostmetering_t without SELinux denials.

    This is the only test exercising the mTLS certificate read:
    unregistered hosts never reach the notify code path, so a missing
    cert_t:file read allow would not surface anywhere else - the daemon
    would simply fail every notification on production systems."""
    offset = _audit_offset()

    _start_service_with_empty_wal()

    # Clear anything earlier tests sent, so an empty query result can
    # only mean that this daemon sent nothing (requires the admin API
    # enabled on the test Prometheus).
    resp = requests.post(
        f"{prometheus_url}/api/v1/admin/tsdb/delete_series",
        params={"match[]": "system_cpu_logical_count"},
        timeout=5,
    )
    assert resp.status_code in (200, 204), (
        f"failed to clear Prometheus series (HTTP {resp.status_code}); "
        "is --web.enable-admin-api enabled on the test Prometheus?"
    )

    # Wait for the daemon to collect and deliver a sample (the default
    # collect interval is on the order of a minute).
    series = []
    deadline = time.time() + 180
    while time.time() < deadline:
        try:
            resp = requests.get(
                f"{prometheus_url}/api/v1/query",
                params={"query": "system_cpu_logical_count"},
                timeout=5,
            )
            if resp.status_code == 200:
                result = resp.json().get("data", {}).get("result", [])
                if result:
                    series = result
                    break
        except requests.RequestException:
            pass
        time.sleep(5)

    assert series, (
        "the registered daemon did not deliver any sample to the "
        "Prometheus receiver within 180s; the mTLS certificate read or "
        "the send may be failing under SELinux; journal:\n"
        + _journal_tail()
    )
    labels = series[0]["metric"]
    assert labels.get("display_name"), (
        f"delivered sample carries no host identity: {labels}"
    )

    # A delivered sample proves the full path ran inside the confined
    # domain: identity via subscription-manager, the cert_t certificate
    # read for mTLS, and the remote write to the receiver.
    denials = _denials_since(offset)
    assert not denials, (
        f"{len(denials)} AVC denial(s) involving hostmetering on a "
        "registered host:\n" + "\n".join(denials)
    )


@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_service_starts_and_stays_running():
    """The service must not crash-loop: with Restart=always and the default
    StartLimitBurst=5 a daemon that core-dumps during startup exhausts its
    restart budget within a few seconds. Sample the state twice, outside
    the window where a restart could hide a crash."""
    _start_service_fresh()

    time.sleep(8)
    state_first = _systemctl("is-active", SERVICE).stdout.strip()
    time.sleep(8)
    state_second = _systemctl("is-active", SERVICE).stdout.strip()
    result = _systemctl("show", "-p", "Result", "--value", SERVICE).stdout.strip()
    restarts = _systemctl("show", "-p", "NRestarts", "--value", SERVICE).stdout.strip()

    assert (state_first, state_second) == ("active", "active"), (
        f"service is not stably active: first={state_first}, "
        f"after 8s more={state_second}, Result={result}, "
        f"NRestarts={restarts}; last journal lines:\n"
        + _systemctl("status", SERVICE, check=False).stdout
    )
    assert restarts == "0", (
        f"service restarted {restarts} times after a fresh start "
        f"(Result={result})"
    )


@pytest.mark.rpm
@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_metrics_wal_survives_restart():
    """RuntimeDirectoryPreserve=yes must keep /run/host-metering (and the
    metrics WAL inside it) across service restarts; systemd deletes the
    runtime directory contents on stop by default, which silently discarded up
    to MetricsMaxAge (90 minutes) of buffered metering data. A reboot still
    clears it: /run is tmpfs. Segments may legitimately disappear when a
    successful notification truncates the samples already delivered, so the
    check is that the WAL is not reset."""
    _start_service_fresh()
    wal_dir = os.path.join(RUNTIME_DIR, "metrics")

    def _segments():
        # Prometheus WAL segments are 20-digit zero-padded file names; other
        # entries (e.g. checkpoint.<n> directories) come and go during
        # truncation and are not part of the invariant.
        if not os.path.isdir(wal_dir):
            return set()
        return {
            name
            for name in os.listdir(wal_dir)
            if re.fullmatch(r"\d{20}", name)
        }

    # Wait for the daemon to write its first sample: systemd creates
    # RUNTIME_DIR before the daemon opens the WAL, so waiting on the directory
    # alone can observe an empty metrics subdirectory.
    deadline = time.time() + 60
    before = set()
    while time.time() < deadline and not before:
        before = _segments()
        if not before:
            time.sleep(1)
    assert before, "no WAL segments under " + wal_dir
    _systemctl("restart", SERVICE)
    deadline = time.time() + 30
    while time.time() < deadline and _main_pid() is None:
        time.sleep(1)
    assert os.path.isdir(wal_dir), (
        "runtime directory was wiped on restart; is "
        "RuntimeDirectoryPreserve=yes set?"
    )
    after = _segments()
    assert after, (
        "the runtime directory was reset on restart: no WAL segments left; "
        "is RuntimeDirectoryPreserve=yes set?"
    )
    # A send truncates the segments below the checkpoint (RemoveSamples after
    # a successful notification), so names may legitimately disappear. A wiped
    # runtime directory instead restarts the WAL from segment 0, which the
    # monotonic high-water mark detects.
    assert max(after) >= max(before), (
        "the WAL restarted from segment 0 on service restart: highest "
        f"segment {max(before)} before, {max(after)} after"
    )


@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed() or not _selinux_module_active(),
    reason="requires installed RPM and enforcing SELinux with the "
    "host-metering module",
)
def test_daemon_runs_in_hostmetering_domain():
    """The daemon process must run in its confined SELinux domain. This is
    the transition that only happens when systemd execs the binary labeled
    hostmetering_exec_t directly (ExecStart), not through a wrapper."""
    _start_service_fresh()
    time.sleep(3)

    pid = _main_pid()
    assert pid, "no main PID: service is not running"

    with open(f"/proc/{pid}/attr/current") as f:
        context = f.read().strip("\x00\n")
    assert context == DAEMON_CONTEXT, (
        f"daemon (pid {pid}) context is {context!r}, expected "
        f"{DAEMON_CONTEXT!r}"
    )


@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed() or not _selinux_module_active(),
    reason="requires installed RPM and enforcing SELinux with the "
    "host-metering module",
)
def test_file_contexts():
    """The policy module file contexts must be applied to the binary, the
    unit file and the runtime directory (which only exists while the
    service is running)."""
    _start_service_fresh()
    time.sleep(3)

    expected = {
        DAEMON: "system_u:object_r:hostmetering_exec_t:s0",
        UNIT_FILE: "system_u:object_r:hostmetering_unit_file_t:s0",
        RUNTIME_DIR: "system_u:object_r:hostmetering_var_run_t:s0",
    }
    problems = []
    for path, want in expected.items():
        got = _stat_context(path)
        if got != want:
            problems.append(f"{path}: {got!r}, expected {want!r}")
    assert not problems, "unexpected SELinux file contexts:\n" + "\n".join(problems)


@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed() or not _selinux_module_active(),
    reason="requires installed RPM and enforcing SELinux with the "
    "host-metering module",
)
def test_no_avc_denials_for_hostmetering():
    """Operating the service must not produce AVC denials involving
    hostmetering_t. The audit log is parsed directly (with a line offset
    taken before the service restart) because ausearch proved unreliable
    on some guests, silently returning no matches while the log contains
    denials; the journal is used as a fallback when auditd is absent."""
    offset = _audit_offset()

    _start_service_fresh()
    time.sleep(10)

    denials = _denials_since(offset)
    if offset is None and shutil.which("journalctl"):
        res = subprocess.run(
            ["journalctl", "-t", "audit", "--no-pager", "-n", "5000"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            universal_newlines=True,
        )
        denials = _audit_denials(res.stdout.splitlines())

    assert not denials, (
        f"{len(denials)} AVC denial(s) involving hostmetering after "
        "service start:\n" + "\n".join(denials)
    )


class _ThreadingHTTPServer(socketserver.ThreadingMixIn, HTTPServer):
    """http.server.ThreadingHTTPServer only exists from Python 3.7 and the
    RHEL 8 lane builds its venv from the guest's Python 3.6, where importing
    it aborts collection for this whole module. The mixin is what the stdlib
    class applies; daemon_threads matches it so server_close() does not wait
    for the handler thread."""

    daemon_threads = True


class _RecordingProxy(BaseHTTPRequestHandler):
    """Minimal HTTP proxy: records the request line and answers 200, so a
    proxied remote write succeeds without forwarding it anywhere."""

    requests = []

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        _RecordingProxy.requests.append((self.command, self.path))
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, *args):
        pass


@pytest.mark.rpm
@pytest.mark.systemd
@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed()
    or not _systemd_available()
    or not _selinux_module_active(),
    reason="requires installed RPM, systemd and enforcing SELinux "
    "(Testing Farm VM)",
)
def test_remote_write_uses_rhsm_proxy(subman_registration):
    """On a registered host the daemon must send its remote write through
    the proxy configured in rhsm.conf: an HTTP proxy sees the absolute
    request URI for the write URL and the notification is accepted, all
    inside hostmetering_t (the SELinux connect to the proxy port is
    covered by the existing squid_port rule).

    subscription-manager identity is a Candlepin call, so the Candlepin
    host stays in no_proxy and reaches the server directly; the recorder
    only serves the daemon's remote write. The write URL is therefore a
    non-local name: an HTTP proxy receives it as an absolute URI, so it
    never needs to resolve. HOST_METERING_WRITE_URL is injected through a
    transient systemd drop-in, the documented way to override service
    configuration."""
    try:
        server = _ThreadingHTTPServer(("127.0.0.1", 3128), _RecordingProxy)
    except OSError as err:
        pytest.skip(f"cannot bind the RHSM proxy port 3128: {err}")
    _RecordingProxy.requests = []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    offset = _audit_offset()
    try:
        with _rhsm_proxy(
            hostname="127.0.0.1",
            port="3128",
            no_proxy="localhost,127.0.0.1",
            env={
                "HOST_METERING_WRITE_URL": PROXY_TEST_WRITE_URL,
                # One failed notification must not hide the proxy path for
                # the whole default 600s interval.
                "HOST_METERING_WRITE_INTERVAL_SEC": "15",
                "HOST_METERING_WRITE_RETRY_ATTEMPTS": "1",
                "HOST_METERING_WRITE_RETRY_MAX_INT_SEC": "2",
                "HOST_METERING_WRITE_TIMEOUT_SEC": "5",
                # The RHSM proxy must win over any ambient proxy.
                "HTTP_PROXY": "",
                "http_proxy": "",
                "HTTPS_PROXY": "",
                "https_proxy": "",
                "NO_PROXY": "",
                "no_proxy": "",
            },
        ):
            pid = _main_pid()
            assert pid, "the service is not running after the proxy restart"
            deadline = time.time() + 90
            while time.time() < deadline and not _RecordingProxy.requests:
                time.sleep(1)
            journal = _journal_for_pid(pid)
            assert _RecordingProxy.requests, (
                "the daemon sent nothing through the RHSM proxy within 90s; "
                "journal:\n" + journal
            )
            deadline = time.time() + 10
            while time.time() < deadline and "Notification successful" not in journal:
                time.sleep(1)
                journal = _journal_for_pid(pid)
            assert "Notification successful" in journal, (
                "the proxied notification was not accepted; journal:\n"
                + journal
            )
        command, path = _RecordingProxy.requests[0]
        assert command == "POST", f"unexpected proxy request: {command} {path}"
        assert path == PROXY_TEST_WRITE_URL, (
            f"the proxy saw {path!r}, expected the absolute write URI "
            f"{PROXY_TEST_WRITE_URL!r}"
        )
        denials = _denials_since(offset)
        assert not denials, (
            f"{len(denials)} AVC denial(s) while using the RHSM proxy:\n"
            + "\n".join(denials)
        )
    finally:
        server.shutdown()
        server.server_close()
