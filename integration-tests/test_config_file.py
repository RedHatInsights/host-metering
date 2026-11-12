"""Tests for the shipped configuration file.

Covers three layers, selected by environment capability via pytest marks:
- everywhere (dev container included): the shipped etc-conf file activates
  configuration when edited;
- ``rpm``: the RPM installs /etc/host-metering.conf (ownership, mode,
  all-comments content);
- ``systemd``: editing /etc/host-metering.conf activates the change after a
  real ``systemctl restart``.

The SELinux behavior of the service reading the shipped
/etc/host-metering.conf is covered by test_selinux_behavior.py, which
starts the stock service under hostmetering_t on real guests.

The container-based dev environment skips the rpm/systemd tests; they run
on Testing Farm, where the tmt wrapper installs the RPM on a VM before
invoking this suite.
"""

import os
import re
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


def test_shipped_config_activates_features(
    tmp_path, run_daemon, cert_generator, prometheus_url
):
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

    # Pin the write target to the session's receiver: the shipped file
    # only documents the default write_url - the production telemetry
    # ingress - and a registered daemon must never send the test
    # guest's telemetry there.
    content += (
        f"\nwrite_url={prometheus_url}/api/v1/write\n"
        f"host_cert_path={cert_path}\n"
        f"host_cert_key_path={key_path}\n"
        f"metrics_wal_path={wal_path}\n"
    )
    with open(config_path, "w") as f:
        f.write(content)

    daemon = run_daemon(config_path)
    try:
        # The custom log formatter drops level prefixes, so DEBUG must be
        # detected via messages that are only emitted at debug level. The
        # metric-collection lines (daemon.go: "Collecting metrics...",
        # "Metrics collected") are emitted on every cycle without a
        # registration, unlike the send-path markers, so the test also
        # passes standalone on an unregistered host.
        debug_markers = ("Collecting metrics", "Metrics collected")
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
def test_config_file_feature_activation_systemd(prometheus_url):
    """Editing /etc/host-metering.conf and restarting the service must
    activate the change: log_level = DEBUG shows up in the journal.

    The write_url is pointed at the session's Prometheus receiver at the
    same time: the shipped default is the production telemetry ingress,
    and this test runs while the suite has the host registered, which
    would otherwise make the service send real telemetry from the test
    guest."""
    with open(INSTALLED_CONF) as f:
        backup = f.read()
    try:
        content = backup
        assert "#log_level = INFO" in content
        content = content.replace("#log_level = INFO", "log_level = DEBUG")
        content = re.sub(
            r"(?m)^#write_url\s*=.*$",
            f"write_url = {prometheus_url}/api/v1/write",
            content,
            count=1,
        )
        with open(INSTALLED_CONF, "w") as f:
            f.write(content)

        # Epoch marker: journalctl --since accepts @<epoch>, which has no
        # one-second-granularity or midnight-wrap window.
        marker = "@%d" % time.time()
        subprocess.run(
            ["systemctl", "restart", "host-metering.service"], check=True
        )
        subprocess.run(
            ["systemctl", "is-active", "--wait", "host-metering.service"],
            check=True,
        )
        time.sleep(5)

        # The daemon logs its whole configuration at startup (main.go,
        # INFO level), registration-independent: a LogLevel of DEBUG can
        # only originate from the edited /etc/host-metering.conf - the
        # test receiver drop-in sets no HOST_METERING_LOG_LEVEL. (The
        # "sample(s)" text must not be used as the marker: it is also
        # emitted at INFO and WARN on the send path.)
        res = subprocess.run(
            [
                "journalctl", "-u", "host-metering",
                "--since", marker, "--no-pager",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
        )
        assert "|  LogLevel: DEBUG" in res.stdout, (
            "log_level = DEBUG did not activate after systemctl restart; "
            f"journal:\n{res.stdout}\n{res.stderr}"
        )
    finally:
        with open(INSTALLED_CONF, "w") as f:
            f.write(backup)
        subprocess.run(
            ["systemctl", "restart", "host-metering.service"],
            check=False,
        )
