import os
import signal
import subprocess
import time


def test_cli_version(host_metering_bin):
    res = subprocess.run(
        [host_metering_bin, "--version"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert res.returncode == 0
    assert "1.4.0" in res.stdout


def test_cli_help(host_metering_bin):
    res = subprocess.run(
        [host_metering_bin, "help"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert res.returncode == 0
    assert "Usage: host-metering" in res.stdout


def test_daemon_graceful_shutdown_sigterm(
    tmp_path, run_daemon, prometheus_url, cert_generator
):
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    config_content = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=10
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=0
label_refresh_interval_sec=10
write_timeout_sec=5
write_retry_attempts=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=30
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    # Start daemon
    dp = run_daemon(config_path)
    time.sleep(2)

    # Stop with SIGTERM gracefully
    dp.stop(signal.SIGTERM)

    # Wait for completion and verify clean exit code
    assert dp.process.returncode == 0
    logs = dp.get_stdout() + dp.get_stderr()
    assert "Server stopped" in logs


def test_daemon_graceful_shutdown_sigint(
    tmp_path, run_daemon, prometheus_url, cert_generator
):
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    config_content = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=10
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=0
label_refresh_interval_sec=10
write_timeout_sec=5
write_retry_attempts=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=30
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    dp = run_daemon(config_path)
    time.sleep(2)

    dp.stop(signal.SIGINT)

    assert dp.process.returncode == 0
    logs = dp.get_stdout() + dp.get_stderr()
    assert "Server stopped" in logs


def test_daemon_sighup_reload(tmp_path, run_daemon, prometheus_url, cert_generator):
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    config_content = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=10
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=0
label_refresh_interval_sec=10
write_timeout_sec=5
write_retry_attempts=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=30
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    dp = run_daemon(config_path)
    time.sleep(2)

    # Send SIGHUP
    os.killpg(os.getpgid(dp.process.pid), signal.SIGHUP)

    # Wait for SIGHUP logs to appear
    reloaded = False
    for _ in range(10):
        logs = dp.get_stdout() + dp.get_stderr()
        if "Reloading HostInfo..." in logs and "HostInfo reloaded" in logs:
            reloaded = True
            break
        time.sleep(0.5)

    dp.stop()
    assert (
        reloaded
    ), f"SIGHUP reload was not logged. Logs:\n{dp.get_stdout() + dp.get_stderr()}"


def test_invalid_configuration_empty_url(tmp_path, host_metering_bin, cert_generator):
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    # Missing write_url
    config_content = f"""[host-metering]
write_url=
write_interval_sec=10
host_cert_path={cert_path}
host_cert_key_path={key_path}
metrics_wal_path={wal_path}
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    env = os.environ.copy()
    env.pop("HOST_METERING_WRITE_URL", None)
    res = subprocess.run(
        [host_metering_bin, "-config", config_path, "daemon"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert res.returncode == 2
    assert "Invalid configuration: WriteURL must be defined" in res.stderr


def test_invalid_configuration_small_write_interval(
    tmp_path, host_metering_bin, cert_generator
):
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)
    cert_generator(cert_path, key_path)

    # WriteInterval too small for retry settings
    config_content = f"""[host-metering]
write_url=http://localhost:9090/api/v1/write
write_interval_sec=1
write_retry_attempts=8
write_retry_max_int_sec=10
write_timeout_sec=60
host_cert_path={cert_path}
host_cert_key_path={key_path}
metrics_wal_path={wal_path}
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    env = os.environ.copy()
    env.pop("HOST_METERING_WRITE_INTERVAL_SEC", None)
    res = subprocess.run(
        [host_metering_bin, "-config", config_path, "daemon"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert res.returncode == 2
    assert "WriteInterval must be bigger than WriteRetryAttempts" in res.stderr
