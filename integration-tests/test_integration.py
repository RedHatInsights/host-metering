import subprocess
import time
import urllib.request
import json
import os
import pytest

def test_daemon_writes_metrics_to_prometheus():
    # Clean any existing WAL or directories
    subprocess.run(["make", "clean"], check=True)
    subprocess.run(["make", "cert"], check=True)
    subprocess.run(["make", "build"], check=True)

    # Start the daemon
    # Ensure we point to Prometheus. Inside the container compose network, "prometheus" is the hostname.
    # Out of container, it could be "localhost". Let's check environment variable HOST_METERING_WRITE_URL
    # or default to http://prometheus:9090/api/v1/write
    write_url = os.environ.get("HOST_METERING_WRITE_URL", "http://prometheus:9090/api/v1/write")
    env = os.environ.copy()
    env["HOST_METERING_WRITE_URL"] = write_url
    env["PATH"] = f"/workspace/host-metering/mocks:{env.get('PATH', '')}"

    proc = subprocess.Popen(
        ["./dist/host-metering", "--config", ".devcontainer/host-metering.conf", "daemon"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )

    try:
        # Let it run for 15 seconds to collect and push some metrics
        time.sleep(15)
        
        # Check if it was running or if it crashed
        ret = proc.poll()
        if ret is not None:
            stdout, stderr = proc.communicate()
            pytest.fail(f"host-metering daemon terminated prematurely with code {ret}.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}")

        # Query Prometheus API to check if system_cpu_logical_count is present
        # The prometheus host is extracted from write_url
        prom_base = write_url.split("/api/v1/write")[0]
        query_url = f"{prom_base}/api/v1/query?query=system_cpu_logical_count"
        
        print(f"Querying Prometheus at {query_url}")
        
        # Let's retry a few times in case of delay
        success = False
        results = []
        for attempt in range(5):
            try:
                req = urllib.request.Request(query_url)
                with urllib.request.urlopen(req, timeout=5) as response:
                    data = json.loads(response.read().decode("utf-8"))
                    if data.get("status") == "success":
                        results = data.get("data", {}).get("result", [])
                        if len(results) > 0:
                            print(f"Metrics found: {results}")
                            success = True
                            break
            except Exception as e:
                print(f"Attempt {attempt+1} failed: {e}")
            time.sleep(3)

        assert success, f"Could not find system_cpu_logical_count metric in Prometheus at {query_url}. Last results: {results}"

    finally:
        # Clean up daemon
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
