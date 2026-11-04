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
- proxy settings written by the ExecStartPre helper into
  /run/host-metering/proxy.env reach the daemon process environment
  through EnvironmentFile=.
"""

import os
import re
import shutil
import subprocess
import time

import pytest
import requests

SERVICE = "host-metering.service"
DAEMON = "/usr/bin/host-metering"
UNIT_FILE = "/usr/lib/systemd/system/host-metering.service"
RUNTIME_DIR = "/run/host-metering"
AUDIT_LOG = "/var/log/audit/audit.log"
RHSM_CONF = "/etc/rhsm/rhsm.conf"
DAEMON_CONTEXT = "system_u:system_r:hostmetering_t:s0"


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


def _candlepin_available():
    """True when the containerized Candlepin server is reachable (it is
    started by tests/integration/test.sh on Testing Farm guests)."""
    host = os.environ.get("CANDLEPIN_HOST", "candlepin")
    port = os.environ.get("CANDLEPIN_PORT", "8443")
    try:
        resp = requests.get(
            f"https://{host}:{port}/candlepin/status",
            verify=False,
            timeout=5,
            allow_redirects=False,
        )
        return resp.status_code == 200
    except requests.RequestException:
        return False


def _prometheus_is_receiver():
    """True when the Prometheus URL points at an actual Prometheus with
    the remote-write receiver API. Guards against port squatters - on
    RHEL cloud images cockpit-tls listens on 9090 and answers 200 HTML
    on any path, which the container script's readiness check cannot
    distinguish from a ready Prometheus."""
    url = os.environ.get("PROMETHEUS_URL", "http://localhost:9090")
    try:
        resp = requests.get(f"{url}/api/v1/status/buildinfo", timeout=5)
        return (
            resp.status_code == 200
            and resp.json().get("status") == "success"
        )
    except (requests.RequestException, ValueError):
        return False


def _journal_tail(since_minutes=3):
    res = subprocess.run(
        ["journalctl", "-u", "host-metering", "--no-pager",
         "--since", f"-{since_minutes}min"],
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


@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_proxy_values_from_rhsm_conf_reach_daemon():
    """The ExecStartPre helper must convert real proxy settings from
    rhsm.conf into the daemon environment, not just empty variables:
    with proxy_hostname/proxy_port set, HTTP_PROXY/HTTPS_PROXY must carry
    the full http://host:port value."""
    assert os.path.exists(RHSM_CONF), f"{RHSM_CONF} not found"
    shutil.copyfile(RHSM_CONF, RHSM_CONF + ".test-backup")
    try:
        with open(RHSM_CONF) as f:
            content = f.read()
        for key, value in (
            ("proxy_hostname", "127.0.0.1"),
            ("proxy_port", "3128"),
        ):
            content, count = re.subn(
                rf"(?m)^{key}\s*=.*$", f"{key} = {value}", content
            )
            assert count == 1, f"expected one {key} line in {RHSM_CONF}"
        with open(RHSM_CONF, "w") as f:
            f.write(content)

        _start_service_fresh()
        time.sleep(3)
        pid = _main_pid()
        assert pid, "no main PID: service is not running"
        with open(f"/proc/{pid}/environ", "rb") as f:
            environ = f.read()

        expected = b"HTTP_PROXY=http://127.0.0.1:3128"
        assert expected in environ and expected.replace(b"HTTP_", b"HTTPS_") in environ, (
            "proxy values from rhsm.conf did not reach the daemon "
            f"environment; environ:\n{environ.decode(errors='replace')}"
        )
    finally:
        shutil.move(RHSM_CONF + ".test-backup", RHSM_CONF)
        _start_service_fresh()


@pytest.mark.rpm
@pytest.mark.systemd
@pytest.mark.selinux
@pytest.mark.skipif(
    not _rpm_installed()
    or not _systemd_available()
    or not _selinux_module_active()
    or not _candlepin_available()
    or not _prometheus_is_receiver(),
    reason="requires installed RPM, systemd, enforcing SELinux, a "
    "reachable Candlepin server and the test Prometheus receiver on "
    "PROMETHEUS_URL (Testing Farm VM; port 9090 must not be squatted "
    "by e.g. cockpit)",
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

    _start_service_fresh()

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


@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_proxy_environment_reaches_daemon():
    """The ExecStartPre helper writes /run/host-metering/proxy.env, which
    the unit loads through EnvironmentFile=; the variables (possibly
    empty, when no proxy is configured) must be visible in the daemon's
    environment, proving the chain is not silently broken."""
    _start_service_fresh()
    time.sleep(3)

    pid = _main_pid()
    assert pid, "no main PID: service is not running"

    with open(f"/proc/{pid}/environ", "rb") as f:
        environ = f.read().split(b"\0")
    names = {entry.split(b"=", 1)[0] for entry in environ if b"=" in entry}
    assert {b"HTTP_PROXY", b"HTTPS_PROXY"} <= names, (
        "HTTP_PROXY/HTTPS_PROXY missing from daemon environment; the "
        "ExecStartPre -> proxy.env -> EnvironmentFile chain is broken"
    )
