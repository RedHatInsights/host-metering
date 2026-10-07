import os
import time


def test_cert_rotation(
    tmp_path, run_daemon, host_metering_bin, mock_env, prometheus_url, cert_generator
):
    # 1. Setup paths
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)

    # 2. Generate initial certificates
    cert_generator(cert_path, key_path, common_name="initial-host.host-metering.test")

    # 3. Create config
    config_content = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=5
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=0
label_refresh_interval_sec=10
write_timeout_sec=1
write_retry_attempts=1
write_retry_min_int_sec=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=30
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content)

    # 4. Start daemon
    dp = run_daemon(config_path)

    # Wait for the daemon to start and initialize
    time.sleep(2)

    logs_before = dp.get_stdout() + dp.get_stderr()
    assert (
        "Watching cert directory" in logs_before
    ), "Daemon should be watching the cert directory"

    # 5. Overwrite certificates on disk (triggering cert rotation / inotify)
    cert_generator(cert_path, key_path, common_name="rotated-host.host-metering.test")

    # 6. Wait for certwatcher / inotify to trigger and reload host info
    triggered = False
    for _ in range(10):
        logs = dp.get_stdout() + dp.get_stderr()
        if (
            "Host cert updated" in logs
            or "Host cert removed" in logs
            or "Host cert loaded" in logs
            or "HostInfo loaded" in logs
        ):
            triggered = True
            break
        time.sleep(0.5)

    dp.stop()

    assert (
        triggered
    ), f"Dynamic reload was not triggered after cert rotation. Logs:\n{dp.get_stdout() + dp.get_stderr()}"
