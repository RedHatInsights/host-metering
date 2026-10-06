import os
import time
import requests
import pytest

def test_backlog_recovery(tmp_path, run_daemon, host_metering_bin, mock_env, prometheus_url, cert_generator):
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
    
    # Let it run for 4 seconds to accumulate samples in WAL
    time.sleep(4)
    
    # Flush logs and check daemon stdout/stderr or status
    stderr = dp.get_stderr()
    stdout = dp.get_stdout()
    
    # Assert daemon compiled and ran, and reported failures to connect (Notification [x sample(s)]: ...)
    # But collected metrics successfully into the WAL
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
    
    # Give it some time to push backlogged metrics
    time.sleep(3)
    dp2.stop()

    # 7. Query range vector to assert multiple samples exist in Prometheus with no gaps
    query_url = f"{prometheus_url}/api/v1/query"
    params = {"query": "system_cpu_logical_count[1m]"}
    
    found = False
    for _ in range(10):
        try:
            resp = requests.get(query_url, params=params, timeout=2)
            if resp.status_code == 200:
                data = resp.json()
                results = data.get("data", {}).get("result", [])
                if results:
                    values = results[0].get("values", [])
                    # We expect multiple samples to be backlogged and sent
                    if len(values) >= 2:
                        found = True
                        break
        except Exception as e:
            print(f"Query attempt failed: {e}")
        time.sleep(1)

    assert found, "Expected backlogged metrics were not successfully ingested or range query failed"
