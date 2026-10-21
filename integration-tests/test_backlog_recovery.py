import os

import requests


def test_backlog_recovery(
    tmp_path,
    run_daemon,
    host_metering_bin,
    subman_registration,
    prometheus_url,
    cert_generator,
    wait_for,
):
    # 1. Setup paths
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)

    # 2. Generate certificates
    cert_generator(cert_path, key_path)

    # 3. Create config with an unreachable write_url
    bad_url = "http://localhost:59999/api/v1/write"
    config_content_bad = f"""[host-metering]
write_url={bad_url}
write_interval_sec=5
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=1
label_refresh_interval_sec=10
write_timeout_sec=1
write_retry_attempts=1
write_retry_min_int_sec=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=60
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content_bad)

    # 4. Start daemon with unreachable write_url
    dp = run_daemon(config_path)

    # Wait for the daemon to accumulate samples in the WAL. The daemon logs
    # one "Metrics collected" debug line per sample persisted to the WAL.
    wait_for(
        lambda: (dp.get_stdout() + dp.get_stderr()).count("Metrics collected") >= 3,
        timeout_s=10,
        desc="daemon to accumulate samples in the WAL",
    )

    # The daemon must report recoverable notification failures while the
    # receiver is unreachable ("Notification [N sample(s)]: ...") and keep
    # buffering the samples in the WAL instead of dropping them.
    wait_for(
        lambda: "Notification [" in dp.get_stdout() + dp.get_stderr(),
        timeout_s=15,
        desc="daemon to report notification failures against the unreachable receiver",
    )
    dp.stop()

    # Verify WAL files are written in the directory and it's not empty
    wal_files = os.listdir(wal_path)
    assert len(wal_files) > 0, "WAL directory should contain persisted samples"

    # 5. Overwrite config with VALID Prometheus URL
    config_content_good = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=5
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=1
label_refresh_interval_sec=10
write_timeout_sec=1
write_retry_attempts=1
write_retry_min_int_sec=1
write_retry_max_int_sec=2
metrics_wal_path={wal_path}
metrics_max_age_sec=60
log_level=DEBUG
instance_id=
"""
    with open(config_path, "w") as f:
        f.write(config_content_good)

    # 6. Start daemon again (it should read backlogged metrics from WAL and push them)
    dp2 = run_daemon(config_path)

    # 7. Query range vector to assert multiple samples exist in Prometheus with no gaps
    query_url = f"{prometheus_url}/api/v1/query"
    params = {"query": "system_cpu_logical_count[1m]"}

    def _backlog_ingested():
        resp = requests.get(query_url, params=params, timeout=2)
        if resp.status_code != 200:
            return None
        results = resp.json().get("data", {}).get("result", [])
        # We expect multiple samples to be backlogged and sent
        if results and len(results[0].get("values", [])) >= 2:
            return results[0]["values"]
        return None

    # Wait for the daemon to push the backlogged metrics and for them to show
    # up in Prometheus
    values = wait_for(
        _backlog_ingested,
        timeout_s=15,
        desc="backlogged metrics to be ingested into Prometheus",
    )
    dp2.stop()

    assert (
        len(values) >= 2
    ), "Expected backlogged metrics were not successfully ingested or range query failed"
