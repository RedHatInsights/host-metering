package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Unset relevant environment variables.
	clearEnvironment()

	// Define the expected defaults.
	expectedCfg := "Config:\n" +
		"|  WriteUrl: http://localhost:9090/api/v1/write\n" +
		"|  WriteIntervalSec: 600\n" +
		"|  HostCertPath: /etc/pki/consumer/cert.pem\n" +
		"|  HostCertKeyPath: /etc/pki/consumer/key.pem\n" +
		"|  CollectIntervalSec: 0\n" +
		"|  LabelRefreshIntervalSec: 86400\n" +
		"|  WriteRetryAttempts: 8\n" +
		"|  WriteRetryMinIntSec: 1\n" +
		"|  WriteRetryMaxIntSec: 10\n" +
		"|  WriteTimeoutSec: 60\n" +
		"|  MetricsMaxAgeSec: 5400\n" +
		"|  MetricsWALPath: /var/run/host-metering/metrics\n" +
		"|  LogLevel: INFO\n" +
		"|  LogPath: \n"

	// Create the default configuration.
	c := NewConfig()
	checkString(t, c.String(), expectedCfg)

	// Environment variables are not set. Keep the defaults.
	err := c.UpdateFromEnvVars()
	checkError(t, err, "failed to update from env variables")
	checkString(t, c.String(), expectedCfg)

	// The configuration file doesn't exist. Keep the defaults.
	dir := t.TempDir()
	path := dir + "/missing"
	_ = c.UpdateFromConfigFile(path)
	checkString(t, c.String(), expectedCfg)

	// The configuration file is empty. Keep the defaults.
	path = dir + "/empty"
	createConfigFile(t, path, "")
	_ = c.UpdateFromConfigFile(path)
	checkString(t, c.String(), expectedCfg)
}

func TestConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/empty"

	// Unset relevant environment variables.
	clearEnvironment()

	// Define the expected configuration.
	expectedCfg := "Config:\n" +
		"|  WriteUrl: http://test/url\n" +
		"|  WriteIntervalSec: 10\n" +
		"|  HostCertPath: /tmp/cert.pem\n" +
		"|  HostCertKeyPath: /tmp/key.pem\n" +
		"|  CollectIntervalSec: 20\n" +
		"|  LabelRefreshIntervalSec: 300\n" +
		"|  WriteRetryAttempts: 4\n" +
		"|  WriteRetryMinIntSec: 5\n" +
		"|  WriteRetryMaxIntSec: 6\n" +
		"|  WriteTimeoutSec: 6\n" +
		"|  MetricsMaxAgeSec: 700\n" +
		"|  MetricsWALPath: /tmp/metrics\n" +
		"|  LogLevel: ERROR\n" +
		"|  LogPath: /tmp/log\n"

	// Update the configuration from a valid config file.
	fileContent := "[host-metering]\n" +
		"# Ignore comments and empty lines.\n\n" +
		"write_url = http://test/url\n" +
		"write_interval_sec = 10\n" +
		"host_cert_path = /tmp/cert.pem\n" +
		"host_cert_key_path = /tmp/key.pem\n" +
		"collect_interval_sec = 20\n" +
		"; And also these comments.\n" +
		"label_refresh_interval_sec = 300\n" +
		"write_retry_attempts = 4\n" +
		"write_retry_min_int_sec = 5\n" +
		"write_retry_max_int_sec = 6\n" +
		"write_timeout_sec = 6\n" +
		"metrics_max_age_sec = 700\n" +
		"metrics_wal_path = /tmp/metrics\n" +
		"log_level = ERROR\n" +
		"log_path = /tmp/log\n"

	c := NewConfig()

	createConfigFile(t, path, fileContent)
	err := c.UpdateFromConfigFile(path)

	checkError(t, err, "failed to update from env variables")
	checkString(t, c.String(), expectedCfg)

	// Don't update the configuration from a invalid config file.
	fileContent = "[host-metering]\n" +
		"write_interval_sec = a\n" +
		"collect_interval_sec = b\n" +
		"label_refresh_interval_sec = c\n" +
		"write_retry_attempts = d\n" +
		"write_retry_min_int_sec = e\n" +
		"write_retry_max_int_sec = f\n" +
		"write_timeout_sec = g\n" +
		"metrics_max_age_sec = h\n"

	createConfigFile(t, path, fileContent)
	err = c.UpdateFromConfigFile(path)

	expectedMsg := "multiple errors occurred:\n" +
		"invalid value of 'write_interval_sec': strconv.ParseUint: parsing \"a\": invalid syntax\n" +
		"invalid value of 'collect_interval_sec': strconv.ParseUint: parsing \"b\": invalid syntax\n" +
		"invalid value of 'label_refresh_interval_sec': strconv.ParseUint: parsing \"c\": invalid syntax\n" +
		"invalid value of 'write_retry_attempts': strconv.ParseUint: parsing \"d\": invalid syntax\n" +
		"invalid value of 'write_retry_min_int_sec': strconv.ParseUint: parsing \"e\": invalid syntax\n" +
		"invalid value of 'write_retry_max_int_sec': strconv.ParseUint: parsing \"f\": invalid syntax\n" +
		"invalid value of 'write_timeout_sec': strconv.ParseUint: parsing \"g\": invalid syntax\n" +
		"invalid value of 'metrics_max_age_sec': strconv.ParseUint: parsing \"h\": invalid syntax\n"

	checkString(t, err.Error(), expectedMsg)
	checkString(t, c.String(), expectedCfg)

}

func TestEnvVariables(t *testing.T) {
	// Unset relevant environment variables.
	clearEnvironment()

	// Define the expected configuration.
	expectedCfg := "Config:\n" +
		"|  WriteUrl: http://test/url\n" +
		"|  WriteIntervalSec: 10\n" +
		"|  HostCertPath: /tmp/cert.pem\n" +
		"|  HostCertKeyPath: /tmp/key.pem\n" +
		"|  CollectIntervalSec: 20\n" +
		"|  LabelRefreshIntervalSec: 300\n" +
		"|  WriteRetryAttempts: 4\n" +
		"|  WriteRetryMinIntSec: 5\n" +
		"|  WriteRetryMaxIntSec: 6\n" +
		"|  WriteTimeoutSec: 6\n" +
		"|  MetricsMaxAgeSec: 700\n" +
		"|  MetricsWALPath: /tmp/metrics\n" +
		"|  LogLevel: ERROR\n" +
		"|  LogPath: /tmp/log\n"

	// Set valid environment variables.
	t.Setenv("HOST_METERING_WRITE_URL", "http://test/url")
	t.Setenv("HOST_METERING_WRITE_INTERVAL_SEC", "10")
	t.Setenv("HOST_METERING_HOST_CERT_PATH", "/tmp/cert.pem")
	t.Setenv("HOST_METERING_HOST_CERT_KEY_PATH", "/tmp/key.pem")
	t.Setenv("HOST_METERING_COLLECT_INTERVAL_SEC", "20")
	t.Setenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC", "300")
	t.Setenv("HOST_METERING_WRITE_RETRY_ATTEMPTS", "4")
	t.Setenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC", "5")
	t.Setenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC", "6")
	t.Setenv("HOST_METERING_WRITE_TIMEOUT_SEC", "6")
	t.Setenv("HOST_METERING_METRICS_MAX_AGE_SEC", "700")
	t.Setenv("HOST_METERING_METRICS_WAL_PATH", "/tmp/metrics")
	t.Setenv("HOST_METERING_LOG_LEVEL", "ERROR")
	t.Setenv("HOST_METERING_LOG_PATH", "/tmp/log")

	// Environment variables are set. Change the defaults.
	c := NewConfig()
	err := c.UpdateFromEnvVars()

	checkError(t, err, "failed to update from env variables")
	checkString(t, c.String(), expectedCfg)

	// Set invalid environment variables.
	t.Setenv("HOST_METERING_WRITE_INTERVAL_SEC", "a")
	t.Setenv("HOST_METERING_COLLECT_INTERVAL_SEC", "b")
	t.Setenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC", "c")
	t.Setenv("HOST_METERING_WRITE_RETRY_ATTEMPTS", "d")
	t.Setenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC", "e")
	t.Setenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC", "f")
	t.Setenv("HOST_METERING_WRITE_TIMEOUT_SEC", "g")
	t.Setenv("HOST_METERING_METRICS_MAX_AGE_SEC", "h")

	// Environment variables are invalid. Keep the previous configuration.
	err = c.UpdateFromEnvVars()

	expectedMsg := "multiple errors occurred:\n" +
		"invalid value of 'HOST_METERING_WRITE_INTERVAL_SEC': strconv.ParseUint: parsing \"a\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_COLLECT_INTERVAL_SEC': strconv.ParseUint: parsing \"b\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_LABEL_REFRESH_INTERVAL_SEC': strconv.ParseUint: parsing \"c\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_WRITE_RETRY_ATTEMPTS': strconv.ParseUint: parsing \"d\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_WRITE_RETRY_MIN_INT_SEC': strconv.ParseUint: parsing \"e\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_WRITE_RETRY_MAX_INT_SEC': strconv.ParseUint: parsing \"f\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_WRITE_TIMEOUT_SEC': strconv.ParseUint: parsing \"g\": invalid syntax\n" +
		"invalid value of 'HOST_METERING_METRICS_MAX_AGE_SEC': strconv.ParseUint: parsing \"h\": invalid syntax\n"

	checkString(t, c.String(), expectedCfg)
	checkString(t, err.Error(), expectedMsg)

}

func clearEnvironment() {
	// Make sure that these environment variables are unset.
	// WARNING: They won't be restored after the test.
	_ = os.Unsetenv("HOST_METERING_WRITE_URL")
	_ = os.Unsetenv("HOST_METERING_WRITE_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_HOST_CERT_PATH")
	_ = os.Unsetenv("HOST_METERING_HOST_CERT_KEY_PATH")
	_ = os.Unsetenv("HOST_METERING_COLLECT_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_ATTEMPTS")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_TIMEOUT_SEC")
	_ = os.Unsetenv("HOST_METERING_METRICS_MAX_AGE_SEC")
	_ = os.Unsetenv("HOST_METERING_METRICS_WAL_PATH")
	_ = os.Unsetenv("HOST_METERING_LOG_LEVEL")
	_ = os.Unsetenv("HOST_METERING_LOG_PATH")
}

func checkError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

func checkString(t *testing.T, s string, expected string) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	expectedLines := strings.Split(strings.TrimSpace(expected), "\n")

	for idx, line := range lines {
		if idx >= len(expectedLines) || line != expectedLines[idx] {
			t.Fatalf("an unexpected string at the line %d:"+
				" '%s'\n%s\n!=\n%s", idx, line, s, expected)
		}
	}
}

func createConfigFile(t *testing.T, path string, content string) string {
	err := os.WriteFile(path, []byte(content), 0666)

	if err != nil {
		t.Fatalf("failed to write to file at %s: %v", path, err)
	}

	return path
}
