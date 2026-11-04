"""Tests for the shipped configuration file.

Covers three layers, selected by environment capability via pytest marks:
- everywhere (dev container included): the shipped etc-conf file activates
  configuration when edited;
- ``rpm``: the RPM installs /etc/host-metering.conf (ownership, mode,
  all-comments content);
- ``systemd``: editing /etc/host-metering.conf activates the change after a
  real ``systemctl restart``;
- ``selinux``: starting the shipped service produces no AVC denials for
  hostmetering_t (e.g. a missing etc_t read allow would surface here).

The container-based dev environment skips the rpm/systemd/selinux tests;
they run on Testing Farm, where the tmt wrapper installs the RPM on a VM
before invoking this suite.
"""

import os
import shutil
import stat
import subprocess
import time

import pytest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SHIPPED_CONF = os.path.join(REPO_ROOT, "etc-conf", "host-metering.conf")
INSTALLED_CONF = "/etc/host-metering.conf"


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
    if not shutil.which("getenforce"):
        return False
    res = subprocess.run(
        ["getenforce"], stdout=subprocess.PIPE, universal_newlines=True
    )
    if res.stdout.strip() == "Disabled":
        return False
    res = subprocess.run(
        ["semodule", "-l"], stdout=subprocess.PIPE, universal_newlines=True
    )
    return any(
        line.startswith("host-metering") for line in res.stdout.splitlines()
    )


def test_shipped_config_activates_features(tmp_path, run_daemon, cert_generator):
    """Editing the shipped config file must take effect: enabling
    log_level = DEBUG makes the daemon emit debug-level output."""
    config_path = os.path.join(tmp_path, "host-metering.conf")
    with open(SHIPPED_CONF) as f:
        content = f.read()
    assert "#log_level = INFO" in content, "shipped file must document log_level"
    content = content.replace("#log_level = INFO", "log_level = DEBUG")

    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    content += (
        f"\nhost_cert_path={cert_path}\n"
        f"host_cert_key_path={key_path}\n"
        f"metrics_wal_path={wal_path}\n"
    )
    with open(config_path, "w") as f:
        f.write(content)

    daemon = run_daemon(config_path)
    try:
        # The custom log formatter drops level prefixes, so DEBUG must be
        # detected via messages that are only emitted at debug level
        # (daemon.go: "Sending N sample(s)...",
        #  prometheus.go: "PrometheusRemoteWrite: Response body:").
        debug_markers = ("sample(s)", "PrometheusRemoteWrite")
        deadline = time.time() + 10
        while time.time() < deadline:
            output = daemon.get_stdout() + daemon.get_stderr()
            if any(marker in output for marker in debug_markers):
                return
            time.sleep(0.5)
        pytest.fail(
            "log_level = DEBUG from the shipped config file did not activate "
            f"debug logging; daemon output:\n{output}"
        )
    finally:
        daemon.stop()


@pytest.mark.rpm
@pytest.mark.skipif(not _rpm_installed(), reason="host-metering RPM not installed")
def test_config_file_installed_by_rpm():
    """The RPM must install /etc/host-metering.conf: correct ownership,
    mode 0644, and no active values (all defaults are commented out)."""
    assert os.path.exists(INSTALLED_CONF), (
        f"{INSTALLED_CONF} missing - RPM does not install the config file"
    )

    res = subprocess.run(
        ["rpm", "-qf", INSTALLED_CONF],
        stdout=subprocess.PIPE,
        universal_newlines=True,
    )
    assert res.returncode == 0, f"no package owns {INSTALLED_CONF}"
    assert res.stdout.strip().startswith("host-metering"), (
        f"{INSTALLED_CONF} owned by unexpected package: {res.stdout.strip()}"
    )

    mode = stat.S_IMODE(os.stat(INSTALLED_CONF).st_mode)
    assert mode == 0o644, f"unexpected mode {oct(mode)} on {INSTALLED_CONF}"

    with open(INSTALLED_CONF) as f:
        lines = f.readlines()
    active = [
        line.strip()
        for line in lines
        if line.strip() and not line.strip().startswith(("#", ";"))
    ]
    assert active == ["[host-metering]"], (
        "shipped config file must not override compiled-in defaults, "
        f"active lines: {active}"
    )


@pytest.mark.systemd
@pytest.mark.skipif(
    not _rpm_installed() or not _systemd_available(),
    reason="requires installed RPM and systemd (Testing Farm VM)",
)
def test_config_file_feature_activation_systemd():
    """Editing /etc/host-metering.conf and restarting the service must
    activate the change: log_level = DEBUG shows up in the journal."""
    shutil.copyfile(INSTALLED_CONF, INSTALLED_CONF + ".test-backup")
    try:
        with open(INSTALLED_CONF) as f:
            content = f.read()
        assert "#log_level = INFO" in content
        with open(INSTALLED_CONF, "w") as f:
            f.write(content.replace("#log_level = INFO", "log_level = DEBUG"))

        marker = time.strftime("%H:%M:%S")
        subprocess.run(
            ["systemctl", "restart", "host-metering.service"], check=True
        )
        subprocess.run(
            ["systemctl", "is-active", "--wait", "host-metering.service"],
            check=True,
        )
        time.sleep(5)

        # The custom log formatter drops level prefixes, so DEBUG must be
        # detected via a message only emitted at debug level (daemon.go:
        # "Sending N sample(s)..."). Fetch the recent journal and look for it.
        res = subprocess.run(
            [
                "journalctl", "-u", "host-metering",
                "--since", marker, "--no-pager",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
        )
        assert "sample(s)" in res.stdout, (
            "log_level = DEBUG did not activate after systemctl restart; "
            f"journal:\n{res.stdout}\n{res.stderr}"
        )
    finally:
        shutil.move(INSTALLED_CONF + ".test-backup", INSTALLED_CONF)
        subprocess.run(
            ["systemctl", "restart", "host-metering.service"],
            check=False,
        )


@pytest.mark.selinux
@pytest.mark.skipif(
    not _selinux_module_active(),
    reason="requires enforcing SELinux with the host-metering module",
)
def test_no_avc_denials_for_hostmetering():
    """Starting the shipped service must not produce AVC denials for
    hostmetering_t - e.g. a missing files_read_etc_files() allow for
    reading /etc/host-metering.conf would surface here."""
    marker = time.strftime("%H:%M:%S")
    subprocess.run(["systemctl", "start", "host-metering.service"], check=True)
    time.sleep(5)
    subprocess.run(["systemctl", "stop", "host-metering.service"], check=False)

    avcs = ""
    if shutil.which("ausearch"):
        res = subprocess.run(
            ["ausearch", "-m", "avc", "-ts", marker, "--raw"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            universal_newlines=True,
        )
        avcs = res.stdout
    if not avcs.strip() and shutil.which("journalctl"):
        # Fall back to the journal if auditd/ausearch is unavailable.
        res = subprocess.run(
            ["journalctl", "-t", "audit", "--since", marker, "--no-pager"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            universal_newlines=True,
        )
        avcs = "\n".join(
            line for line in res.stdout.splitlines() if "avc:" in line and "denied" in line
        )

    denials = [line for line in avcs.splitlines() if "hostmetering_t" in line]
    assert not denials, (
        "AVC denials involving hostmetering_t after service start:\n"
        + "\n".join(denials)
    )
