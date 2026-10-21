import os

import requests


def test_unregistered_host_does_not_send(
    tmp_path,
    run_daemon,
    subman_registration,
    prometheus_url,
    cert_generator,
    wait_for,
):
    """A host without a registration must not send anything.

    The daemon keeps collecting samples into the WAL, but the notify
    policy blocks on the missing identity until the system registers.
    This pins the registration gate end-to-end against the real
    subscription-manager.
    """
    # Drop the session registration; the real subscription-manager must
    # report the system as unregistered and the consumer cert must be gone.
    subman_registration.unregister()
    try:
        assert not subman_registration.registered
        assert not os.path.exists("/etc/pki/consumer/cert.pem")

        # Clear any series earlier tests sent, so an empty query result can
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

        # Run the daemon with a short write interval.
        config_path = os.path.join(tmp_path, "host-metering.conf")
        cert_path = os.path.join(tmp_path, "cert.pem")
        key_path = os.path.join(tmp_path, "key.pem")
        wal_path = os.path.join(tmp_path, "wal")
        os.makedirs(wal_path, exist_ok=True)
        cert_generator(cert_path, key_path)

        config_content = f"""[host-metering]
write_url={prometheus_url}/api/v1/write
write_interval_sec=2
host_cert_path={cert_path}
host_cert_key_path={key_path}
collect_interval_sec=0
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
            f.write(config_content)

        dp = run_daemon(config_path)

        # Samples keep accumulating in the WAL while unregistered.
        wait_for(
            lambda: (dp.get_stdout() + dp.get_stderr()).count("Metrics collected")
            >= 2,
            timeout_s=15,
            desc="daemon to collect samples while unregistered",
        )
        # ... and at least one write interval must have been blocked.
        wait_for(
            lambda: "Cannot notify: missing HostId"
            in dp.get_stdout() + dp.get_stderr(),
            timeout_s=15,
            desc="notify policy to block on the missing identity",
        )

        logs = dp.get_stdout() + dp.get_stderr()
        assert (
            "Notification successful" not in logs
        ), f"daemon sent data while unregistered:\n{logs}"
        dp.stop()

        # Nothing may have reached Prometheus.
        resp = requests.get(
            f"{prometheus_url}/api/v1/query",
            params={"query": "system_cpu_logical_count"},
            timeout=5,
        )
        assert resp.status_code == 200
        results = resp.json().get("data", {}).get("result", [])
        assert (
            results == []
        ), f"metrics appeared in Prometheus while unregistered: {results}"
    finally:
        # Restore the session registration for any remaining tests.
        subman_registration.register()
