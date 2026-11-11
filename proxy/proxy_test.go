package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/RedHatInsights/host-metering/config"
)

func checkError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

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
	resolve := func(t *testing.T, c *config.Config, rawUrl string) *url.URL {
		proxyUrl, err := ProxyFunc(c)(newRequest(rawUrl))
		if err != nil {
			t.Fatalf("failed to resolve the proxy for %s: %v", rawUrl, err)
		}
		return proxyUrl
	}

	// No RHSM proxy: the environment decides, and it is empty here.
	c := config.NewConfig()
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

	// NO_PROXY from the service environment also excludes targets from the
	// rhsm.conf proxy.
	c.NoProxy = ""
	t.Setenv("NO_PROXY", "write.example.test")
	if proxyUrl := resolve(t, c, "http://write.example.test/api/v1/write"); proxyUrl != nil {
		t.Fatalf("expected the environment NO_PROXY to exclude the target, got %s", proxyUrl)
	}
	t.Setenv("NO_PROXY", "")

	// Localhost and loopback targets are never sent to the rhsm.conf proxy.
	for _, target := range []string{
		"http://localhost/api/v1/write",
		"http://127.0.0.1/api/v1/write",
		"http://[::1]/api/v1/write",
	} {
		if proxyUrl := resolve(t, c, target); proxyUrl != nil {
			t.Fatalf("expected %s not to be proxied, got %s", target, proxyUrl)
		}
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
		{"*.example.test", "http://write.example.test/api/v1/write", false},
		{"WRITE.Example.TEST", "http://write.example.test/api/v1/write", false},
		{" other.test , write.example.test ", "http://write.example.test/api/v1/write", false},
		{"write.example.test:8080", "http://write.example.test/api/v1/write", true},
		{"write.example.test:80", "http://write.example.test/api/v1/write", false},
		{"write.example.test:8080", "http://write.example.test:8080/api/v1/write", false},
		{"10.0.0.0/8", "http://10.1.2.3/api/v1/write", false},
		{"10.0.0.0/8", "http://11.1.2.3/api/v1/write", true},
		// not an IP or CIDR: net/http treats it as a domain suffix
		{"0.0.1", "http://10.0.0.1/api/v1/write", false},
		{"10.0.0.1", "http://10.0.0.1/api/v1/write", false},
		{"[::1]", "http://[::1]/api/v1/write", false},
		{"::1", "http://[::1]/api/v1/write", false},
		{"[::1]", "http://[::2]/api/v1/write", true},
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

	// An unusable proxy url must fail requests, not go silently unproxied.
	c.ProxyUrl = "not a url"
	c.NoProxy = ""
	if proxyUrl, err := ProxyFunc(c)(newRequest("http://write.example.test/api/v1/write")); err == nil {
		t.Fatalf("expected an error for an unparsable proxy url, got %v", proxyUrl)
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

	c := config.NewConfig()
	c.ProxyUrl = proxy.URL
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{Proxy: ProxyFunc(c)},
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

// proxyEnvChildEnv marks a re-executed child of a proxy environment test.
const proxyEnvChildEnv = "HOST_METERING_PROXY_TEST_CHILD"

// runProxyEnvChild re-executes the current test binary with a controlled proxy
// environment: http.ProxyFromEnvironment reads the environment once per
// process, so environment precedence is only observable in a fresh process.
// Duplicate keys are fine: os/exec keeps the last value of each variable.
func runProxyEnvChild(t *testing.T, extraEnv map[string]string) {
	t.Helper()
	env := append(os.Environ(),
		proxyEnvChildEnv+"=1",
		"HTTP_PROXY=", "http_proxy=",
		"HTTPS_PROXY=", "https_proxy=",
		"NO_PROXY=", "no_proxy=",
	)
	for name, value := range extraEnv {
		env = append(env, name+"="+value)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child process failed: %v\n%s", err, out)
	}
}

// Test that a configured rhsm.conf proxy wins over the environment for both
// schemes: a lone HTTP_PROXY or HTTPS_PROXY changes nothing.
func TestProxyFuncIgnoresEnvWithRhsmProxy(t *testing.T) {
	if os.Getenv(proxyEnvChildEnv) == "1" {
		c := config.NewConfig()
		c.ProxyUrl = "http://proxy.example.com:3128"
		for _, target := range []string{
			"http://write.example.test/api/v1/write",
			"https://write.example.test/api/v1/write",
		} {
			request, err := http.NewRequest("GET", target, nil)
			checkError(t, err, "failed to create http request")
			proxyUrl, err := ProxyFunc(c)(request)
			checkError(t, err, "failed to resolve the proxy")
			if proxyUrl == nil || proxyUrl.String() != c.ProxyUrl {
				t.Fatalf("%s: expected the RHSM proxy %s, got %v",
					target, c.ProxyUrl, proxyUrl)
			}
		}
		return
	}
	for _, extraEnv := range []map[string]string{
		{"HTTP_PROXY": "http://env-proxy.example.com:8080"},
		{"HTTPS_PROXY": "https://env-proxy.example.com:8443"},
	} {
		runProxyEnvChild(t, extraEnv)
	}
}

// Test that the environment proxy is used when rhsm.conf configures none.
func TestProxyFuncEnvFallback(t *testing.T) {
	if os.Getenv(proxyEnvChildEnv) == "1" {
		c := config.NewConfig()
		request, err := http.NewRequest("GET", "http://write.example.test/api/v1/write", nil)
		checkError(t, err, "failed to create http request")
		proxyUrl, err := ProxyFunc(c)(request)
		checkError(t, err, "failed to resolve the proxy")
		if proxyUrl == nil || proxyUrl.String() != "http://env-proxy.example.com:8080" {
			t.Fatalf("expected the environment proxy, got %v", proxyUrl)
		}
		return
	}
	runProxyEnvChild(t, map[string]string{
		"HTTP_PROXY": "http://env-proxy.example.com:8080",
	})
}
