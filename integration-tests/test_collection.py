import os
import subprocess
import time

import requests


def test_metric_collection_once(
    tmp_path, host_metering_bin, mock_env, prometheus_url, cert_generator
):
    # 1. Setup paths
    config_path = os.path.join(tmp_path, "host-metering.conf")
    cert_path = os.path.join(tmp_path, "cert.pem")
    key_path = os.path.join(tmp_path, "key.pem")
    wal_path = os.path.join(tmp_path, "wal")
    os.makedirs(wal_path, exist_ok=True)

    # 2. Generate certificates
    cert_generator(cert_path, key_path)

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

    # 4. Run once
    env = mock_env.copy()
    # Let's override write url in env if needed, or just let it use the config
    res = subprocess.run(
        [host_metering_bin, "-config", config_path, "once"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert res.returncode == 0, f"daemon failed: {res.stderr}"

    # 5. Query Prometheus with retries
    query_url = f"{prometheus_url}/api/v1/query"
    params = {"query": "system_cpu_logical_count"}

    # Wait up to 10 seconds for the metric to show up
    found = False
    for _ in range(10):
        try:
            resp = requests.get(query_url, params=params, timeout=2)
            if resp.status_code == 200:
                data = resp.json()
                results = data.get("data", {}).get("result", [])
                if results:
                    metric = results[0]["metric"]
                    # Assert expected labels
                    assert metric.get("_id") == "01234567-89ab-cdef-0123-456789abcdef"
                    assert metric.get("display_name") == "host.mock.test"
                    assert metric.get("socket_count") == "3"
                    assert metric.get("product") == "394,69"
                    assert metric.get("billing_marketplace") == "aws"
                    assert metric.get("usage") == "Production"
                    assert metric.get("support") == "Premium"
                    found = True
                    break
        except (requests.RequestException, ValueError, KeyError, AssertionError) as e:
            print(f"Query attempt failed: {e}")
        time.sleep(1)

    assert (
        found
    ), "Expected metric 'system_cpu_logical_count' was not found in Prometheus or label assertion failed"
