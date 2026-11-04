package config

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Test that the proxy resolver follows the documented precedence and the RHSM
// settings.
//
//   - this test must clear HTTP_PROXY/HTTPS_PROXY/NO_PROXY before the first
//     ProxyFunc call: http.ProxyFromEnvironment snapshots the environment once
//     per process, so it has to run before any other test in this package that
//     resolves a proxy.
func TestProxyFunc(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	newRequest := func(rawUrl string) *http.Request {
		request, err := http.NewRequest("GET", rawUrl, nil)
		checkError(t, err, "failed to create http request")
		return request
	}
	resolve := func(t *testing.T, c *Config, rawUrl string) *url.URL {
		proxyUrl, err := c.ProxyFunc()(newRequest(rawUrl))
		if err != nil {
			t.Fatalf("failed to resolve the proxy for %s: %v", rawUrl, err)
		}
		return proxyUrl
	}

	// No RHSM proxy: the environment decides, and it is empty here.
	c := NewConfig()
	if proxyUrl := resolve(t, c, "http://write.example.test/api/v1/write"); proxyUrl != nil {
		t.Fatalf("expected no proxy without an RHSM proxy, got %s", proxyUrl)
	}

	// Both http and https targets go through the RHSM proxy, credentials
	// included.
	c.ProxyUrl = "http://proxy.example.com:3128"
	c.ProxyUser = "proxyuser"
	c.ProxyPassword = "proxypassword"
	c.NoProxy = ""
	for _, target := range []string{
		"http://write.example.test/api/v1/write",
		"https://write.example.test/api/v1/write",
	} {
		proxyUrl := resolve(t, c, target)
		if proxyUrl == nil {
			t.Fatalf("expected %s to be proxied", target)
		}
		if proxyUrl.String() != "http://proxyuser:proxypassword@proxy.example.com:3128" {
			t.Fatalf("unexpected proxy url for %s: %s", target, proxyUrl)
		}
	}

	// A user without a password is sent without one.
	c.ProxyPassword = ""
	if proxyUrl := resolve(t, c, "http://write.example.test/"); proxyUrl.String() != "http://proxyuser@proxy.example.com:3128" {
		t.Fatalf("unexpected proxy url: %s", proxyUrl)
	}

	// no_proxy matching.
	noProxyCases := []struct {
		noProxy string
		target  string
		proxied bool
	}{
		{"", "http://write.example.test/api/v1/write", true},
		{"write.example.test", "http://write.example.test/api/v1/write", false},
		{"example.test", "http://write.example.test/api/v1/write", false},
		{"example.test", "http://other.test/api/v1/write", true},
		{".example.test", "http://write.example.test/api/v1/write", false},
		{".example.test", "http://example.test/api/v1/write", true},
		{"*", "http://write.example.test/api/v1/write", false},
		{"WRITE.Example.TEST", "http://write.example.test/api/v1/write", false},
		{" other.test , write.example.test ", "http://write.example.test/api/v1/write", false},
		{"write.example.test:8080", "http://write.example.test/api/v1/write", true},
		{"write.example.test:80", "http://write.example.test/api/v1/write", false},
		{"write.example.test:8080", "http://write.example.test:8080/api/v1/write", false},
	}
	c.NoProxy = ""
	for _, tc := range noProxyCases {
		c.NoProxy = tc.noProxy
		proxyUrl := resolve(t, c, tc.target)
		if tc.proxied && proxyUrl == nil {
			t.Fatalf("no_proxy %q: expected %s to be proxied", tc.noProxy, tc.target)
		}
		if !tc.proxied && proxyUrl != nil {
			t.Fatalf("no_proxy %q: expected %s not to be proxied, got %s", tc.noProxy, tc.target, proxyUrl)
		}
	}

	// An unusable proxy url falls back to the environment resolver.
	c.ProxyUrl = "not a url"
	c.NoProxy = ""
	if proxyUrl := resolve(t, c, "http://write.example.test/api/v1/write"); proxyUrl != nil {
		t.Fatalf("expected no proxy for an unparsable proxy url, got %s", proxyUrl)
	}
}

// Test that a remote write really reaches the configured proxy: the request
// line of a proxied HTTP request carries the absolute target URI.
func TestProxyFuncRequests(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	var requestUris []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestUris = append(requestUris, r.RequestURI)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()

	c := NewConfig()
	c.ProxyUrl = proxy.URL
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{Proxy: c.ProxyFunc()},
	}
	resp, err := client.Post("http://write.example.test/api/v1/write", "application/x-protobuf", strings.NewReader("metrics"))
	checkError(t, err, "the remote write through the proxy failed")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code through the proxy: %d", resp.StatusCode)
	}

	expected := "http://write.example.test/api/v1/write"
	if len(requestUris) != 1 || requestUris[0] != expected {
		t.Fatalf("expected the proxy to receive a single request for %s, got %v", expected, requestUris)
	}
}

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
		"|  SendHostname: yes\n" +
		"|  WriteRetryAttempts: 8\n" +
		"|  WriteRetryMinIntSec: 1\n" +
		"|  WriteRetryMaxIntSec: 10\n" +
		"|  WriteTimeoutSec: 60\n" +
		"|  MetricsMaxAgeSec: 5400\n" +
		"|  MetricsWALPath: /var/run/host-metering/metrics\n" +
		"|  LogLevel: INFO\n" +
		"|  LogPath: \n" +
		"|  InstanceID: \n" +
		"|  ProxyUrl: \n" +
		"|  NoProxy: \n"

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
		"|  SendHostname: no\n" +
		"|  WriteRetryAttempts: 4\n" +
		"|  WriteRetryMinIntSec: 5\n" +
		"|  WriteRetryMaxIntSec: 6\n" +
		"|  WriteTimeoutSec: 6\n" +
		"|  MetricsMaxAgeSec: 700\n" +
		"|  MetricsWALPath: /tmp/metrics\n" +
		"|  LogLevel: ERROR\n" +
		"|  LogPath: /tmp/log\n" +
		"|  InstanceID: test-instance\n" +
		"|  ProxyUrl: \n" +
		"|  NoProxy: \n"

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
		"send_hostname = no\n" +
		"write_retry_attempts = 4\n" +
		"write_retry_min_int_sec = 5\n" +
		"write_retry_max_int_sec = 6\n" +
		"write_timeout_sec = 6\n" +
		"metrics_max_age_sec = 700\n" +
		"metrics_wal_path = /tmp/metrics\n" +
		"log_level = ERROR\n" +
		"log_path = /tmp/log\n" +
		"instance_id = test-instance\n"

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
		"|  SendHostname: no\n" +
		"|  WriteRetryAttempts: 4\n" +
		"|  WriteRetryMinIntSec: 5\n" +
		"|  WriteRetryMaxIntSec: 6\n" +
		"|  WriteTimeoutSec: 6\n" +
		"|  MetricsMaxAgeSec: 700\n" +
		"|  MetricsWALPath: /tmp/metrics\n" +
		"|  LogLevel: ERROR\n" +
		"|  LogPath: /tmp/log\n" +
		"|  InstanceID: test-instance\n" +
		"|  ProxyUrl: \n" +
		"|  NoProxy: \n"

	// Set valid environment variables.
	t.Setenv("HOST_METERING_WRITE_URL", "http://test/url")
	t.Setenv("HOST_METERING_WRITE_INTERVAL_SEC", "10")
	t.Setenv("HOST_METERING_HOST_CERT_PATH", "/tmp/cert.pem")
	t.Setenv("HOST_METERING_HOST_CERT_KEY_PATH", "/tmp/key.pem")
	t.Setenv("HOST_METERING_COLLECT_INTERVAL_SEC", "20")
	t.Setenv("HOST_METERING_SEND_HOSTNAME", "no")
	t.Setenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC", "300")
	t.Setenv("HOST_METERING_WRITE_RETRY_ATTEMPTS", "4")
	t.Setenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC", "5")
	t.Setenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC", "6")
	t.Setenv("HOST_METERING_WRITE_TIMEOUT_SEC", "6")
	t.Setenv("HOST_METERING_METRICS_MAX_AGE_SEC", "700")
	t.Setenv("HOST_METERING_METRICS_WAL_PATH", "/tmp/metrics")
	t.Setenv("HOST_METERING_LOG_LEVEL", "ERROR")
	t.Setenv("HOST_METERING_LOG_PATH", "/tmp/log")
	t.Setenv("HOST_METERING_INSTANCE_ID", "test-instance")

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

func TestRhsmConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/rhsm.conf"

	// A missing file is not an error: the proxy is optional.
	c := NewConfig()
	err := c.UpdateFromRhsmConfig(dir + "/missing")
	checkError(t, err, "a missing RHSM config file must not be an error")
	if c.ProxyUrl != "" || c.ProxyUser != "" || c.ProxyPassword != "" || c.NoProxy != "" {
		t.Fatalf("unexpected proxy configuration: %+v", c)
	}

	// An empty file and a file without proxy keys leave the proxy unset.
	for _, content := range []string{
		"",
		"[server]\nhostname = localhost\n",
		"[server]\nproxy_hostname = \nproxy_port = 3128\n",
	} {
		createConfigFile(t, path, content)
		c = NewConfig()
		err = c.UpdateFromRhsmConfig(path)
		checkError(t, err, "an empty RHSM proxy configuration must not be an error")
		if c.ProxyUrl != "" || c.ProxyUser != "" || c.ProxyPassword != "" || c.NoProxy != "" {
			t.Fatalf("unexpected proxy configuration for %q: %+v", content, c)
		}
	}

	// A proxy hostname and port, with credentials and a no_proxy list.
	createConfigFile(t, path, "[server]\n"+
		"hostname = localhost\n"+
		"proxy_hostname = proxy.example.com\n"+
		"proxy_port = 3128\n"+
		"proxy_user = proxyuser\n"+
		"proxy_password = proxypassword\n"+
		"no_proxy = localhost, .example.test\n")
	c = NewConfig()
	err = c.UpdateFromRhsmConfig(path)
	checkError(t, err, "failed to read the RHSM proxy configuration")
	if c.ProxyUrl != "http://proxy.example.com:3128" {
		t.Fatalf("unexpected proxy url: %s", c.ProxyUrl)
	}
	if c.ProxyUser != "proxyuser" {
		t.Fatalf("unexpected proxy user: %s", c.ProxyUser)
	}
	if c.ProxyPassword != "proxypassword" {
		t.Fatalf("unexpected proxy password: %s", c.ProxyPassword)
	}
	if c.NoProxy != "localhost, .example.test" {
		t.Fatalf("unexpected no_proxy: %s", c.NoProxy)
	}
	// The password must never end up in the logged configuration.
	if strings.Contains(c.String(), "proxypassword") {
		t.Fatalf("the password is printed in the configuration:\n%s", c.String())
	}

	// proxy_username is accepted as an alias of proxy_user.
	createConfigFile(t, path, "[server]\n"+
		"proxy_hostname = proxy.example.com\n"+
		"proxy_port = 3128\n"+
		"proxy_username = aliasuser\n")
	c = NewConfig()
	err = c.UpdateFromRhsmConfig(path)
	checkError(t, err, "failed to read the RHSM proxy configuration")
	if c.ProxyUser != "aliasuser" {
		t.Fatalf("unexpected proxy user: %s", c.ProxyUser)
	}

	// Only the [server] section is read.
	createConfigFile(t, path, "[rhsm]\n"+
		"proxy_hostname = proxy.example.com\n"+
		"proxy_port = 3128\n")
	c = NewConfig()
	err = c.UpdateFromRhsmConfig(path)
	checkError(t, err, "failed to read the RHSM config file")
	if c.ProxyUrl != "" {
		t.Fatalf("expected the [rhsm] section to be ignored, got %s", c.ProxyUrl)
	}

	// Invalid proxy configurations are reported.
	invalidCases := []struct {
		name    string
		content string
		message string
	}{
		{
			name:    "missing port",
			content: "[server]\nproxy_hostname = proxy.example.com\n",
			message: "proxy_port is missing",
		},
		{
			name:    "non numeric port",
			content: "[server]\nproxy_hostname = proxy.example.com\nproxy_port = http\n",
			message: "invalid value of 'proxy_port'",
		},
		{
			name:    "zero port",
			content: "[server]\nproxy_hostname = proxy.example.com\nproxy_port = 0\n",
			message: "invalid value of 'proxy_port'",
		},
		{
			name:    "out of range port",
			content: "[server]\nproxy_hostname = proxy.example.com\nproxy_port = 70000\n",
			message: "invalid value of 'proxy_port'",
		},
		{
			name:    "scheme in hostname",
			content: "[server]\nproxy_hostname = http://proxy.example.com\nproxy_port = 3128\n",
			message: "must not contain a scheme",
		},
	}
	for _, tc := range invalidCases {
		createConfigFile(t, path, tc.content)
		c = NewConfig()
		err = c.UpdateFromRhsmConfig(path)
		if err == nil {
			t.Fatalf("%s: expected an error for %q", tc.name, tc.content)
		}
		if !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

func clearEnvironment() {
	// Make sure that these environment variables are unset.
	// WARNING: They won't be restored after the test.
	_ = os.Unsetenv("HOST_METERING_WRITE_URL")
	_ = os.Unsetenv("HOST_METERING_WRITE_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_HOST_CERT_PATH")
	_ = os.Unsetenv("HOST_METERING_HOST_CERT_KEY_PATH")
	_ = os.Unsetenv("HOST_METERING_COLLECT_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_SEND_HOSTNAME")
	_ = os.Unsetenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_ATTEMPTS")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC")
	_ = os.Unsetenv("HOST_METERING_WRITE_TIMEOUT_SEC")
	_ = os.Unsetenv("HOST_METERING_METRICS_MAX_AGE_SEC")
	_ = os.Unsetenv("HOST_METERING_METRICS_WAL_PATH")
	_ = os.Unsetenv("HOST_METERING_LOG_LEVEL")
	_ = os.Unsetenv("HOST_METERING_LOG_PATH")
	_ = os.Unsetenv("HOST_METERING_INSTANCE_ID")
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
