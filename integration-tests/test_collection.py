import os
import subprocess

import requests


def test_metric_collection_once(
    tmp_path,
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
    env = os.environ.copy()
    # Let's override write url in env if needed, or just let it use the config
    res = subprocess.run(
        [host_metering_bin, "-config", config_path, "once"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=False,
    )
    assert res.returncode == 0, f"daemon failed: {res.stderr}"

    # 5. Query Prometheus with retries
    query_url = f"{prometheus_url}/api/v1/query"
    params = {"query": "system_cpu_logical_count"}

    def _metric_with_expected_labels():
        resp = requests.get(query_url, params=params, timeout=2)
        if resp.status_code != 200:
            return None
        results = resp.json().get("data", {}).get("result", [])
        if not results:
            return None
        metric = results[0]["metric"]
        # Assert labels derived from the real registration: the identity
        # (dynamic per session), the facts of the test container, and the
        # syspurpose values set by the registration fixture.
        assert metric.get("_id") == subman_registration.identity.get(
            "system identity"
        ), f"_id label does not match registered identity: {metric}"
        assert metric.get("display_name") == subman_registration.identity.get(
            "name"
        ), f"display_name label does not match registered name: {metric}"
        assert metric.get("socket_count") == subman_registration.facts.get(
            "cpu.cpu_socket(s)"
        ), f"socket_count label does not match facts: {metric}"

        def _assert_syspurpose_label(label, field):
            expected = subman_registration.syspurpose.get(field, "")
            if expected:
                assert metric.get(label) == expected, (
                    f"{label} label does not match syspurpose value "
                    f"{expected!r}: {metric}"
                )
            else:
                assert label not in metric, (
                    f"{label} label should be absent for empty syspurpose "
                    f"value: {metric}"
                )

        _assert_syspurpose_label("usage", "usage")
        _assert_syspurpose_label("support", "service_level_agreement")

        # Hosts can be cloud instances (the CentOS Stream TF runs are AWS
        # EC2) and RHEL guests carry product certificates. Derive the
        # optional labels from the same subscription-manager data the
        # daemon reads.
        expected_products = ",".join(subman_registration.products)
        if expected_products:
            assert metric.get("product") == expected_products, (
                f"product label does not match `subscription-manager list "
                f"--installed` {expected_products!r}: {metric}"
            )
        else:
            assert "product" not in metric, f"unexpected product label: {metric}"

        def _assert_fact_label(label, expected):
            if expected:
                assert metric.get(label) == expected, (
                    f"{label} label does not match subscription-manager fact "
                    f"{expected!r}: {metric}"
                )
            else:
                assert label not in metric, (
                    f"{label} label should be absent for an empty fact: {metric}"
                )

        facts = subman_registration.facts
        billing = {}
        if "aws_instance_id" in facts:
            billing = {
                "billing_marketplace": "aws",
                "billing_marketplace_account": facts.get("aws_account_id", ""),
                "billing_marketplace_instance_id": facts["aws_instance_id"],
            }
        elif "azure_instance_id" in facts:
            billing = {
                "billing_marketplace": "azure",
                "billing_marketplace_account": facts.get("azure_subscription_id", ""),
                "billing_marketplace_instance_id": facts["azure_instance_id"],
            }
        elif "gcp_instance_id" in facts:
            billing = {
                "billing_marketplace": "gcp",
                "billing_marketplace_account": facts.get("gcp_project_number", ""),
                "billing_marketplace_instance_id": facts["gcp_instance_id"],
            }
        if billing:
            for label, expected in billing.items():
                _assert_fact_label(label, expected)
            _assert_fact_label("billing_model", "marketplace")
        else:
            for label in (
                "billing_marketplace",
                "billing_marketplace_account",
                "billing_marketplace_instance_id",
                "billing_model",
            ):
                _assert_fact_label(label, "")
        return metric

    # Wait up to 10 seconds for the metric to show up
    metric = wait_for(
        _metric_with_expected_labels,
        timeout_s=10,
        interval_s=1,
        desc="metric 'system_cpu_logical_count' with expected labels to show up in Prometheus",
    )

    assert (
        metric
    ), "Expected metric 'system_cpu_logical_count' was not found in Prometheus or label assertion failed"
