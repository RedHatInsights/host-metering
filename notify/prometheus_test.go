package notify

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/RedHatInsights/host-metering/config"
	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

const (
	writeUrlPath = "/api/v1/write"
	testHostname = "host.example.test"
)

// Test that http client follows the environment Proxy settings
//
//   - this test is fragile as it is influenced by the environment as ProxyFromEnvironment
//     is initialized only once, thus if any test before uses it without the env vars set then
//     this will fail.
func TestHttpClientProxy(t *testing.T) {
	// Init
	keypair, _, _, _ := createTestKeypair(t)
	httpProxy := "http://proxy.example.com"
	httpsProxy := "https://proxy.example.com"

	t.Setenv("HTTP_PROXY", httpProxy)
	t.Setenv("HTTPS_PROXY", httpsProxy)
	client, err := newMTLSHttpClient(keypair, 1*time.Second)
	checkError(t, err, "Failed to create http client")
	proxyF := client.Transport.(*http.Transport).Proxy
	if proxyF == nil {
		t.Fatalf("Expected proxy function to be set")
	}

	// Test https proxy
	httpsRequest, _ := http.NewRequest("GET", "https://example.com", nil)
	checkError(t, err, "Failed to create http request")
	proxyUrl, _ := proxyF(httpsRequest)
	checkError(t, err, "Failed to get proxy url")
	if proxyUrl == nil {
		t.Fatalf("Expected proxy url to be set")
	}
	if proxyUrl.String() != httpsProxy {
		t.Fatalf("Expected https proxy to be %s, got %s", httpsProxy, proxyUrl.String())
	}

	// Test http proxy
	httpRequest, _ := http.NewRequest("GET", "http://example.com", nil)
	proxyUrl, _ = proxyF(httpRequest)
	if proxyUrl.String() != httpProxy {
		t.Fatalf("Expected http proxy to be %s, got %s", httpProxy, proxyUrl.String())
	}
}

// Test happy path of prometheus notifier
func TestNotify(t *testing.T) {
	// Initialize notifier and data
	useInsecureTLS(t)
	_, certPath, keyPath, _ := createTestKeypair(t)
	cfg := &config.Config{
		HostCertPath:       certPath,
		HostCertKeyPath:    keyPath,
		WriteRetryAttempts: 1,
	}
	n := NewPrometheusNotifier(cfg)
	samples := createSamples()
	hostinfo := createHostInfo()

	// Initialize mock server
	called := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called += 1
		if r.URL.Path != writeUrlPath {
			t.Errorf("Expected to request '%s', got: %s", writeUrlPath, r.URL.Path)
		}

		// Test that request is in prometheus remote write format
		checkPromethuesRemoteWriteHeaders(t, r)
		checkRequestBody(t, r)
		checkRequestUsesHostCert(t, r)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte{})
	}))
	server.TLS = &tls.Config{
		ClientAuth: tls.RequireAnyClientCert,
	}
	server.StartTLS()
	defer server.Close()
	cfg.WriteUrl = server.URL + writeUrlPath

	// Test that notify returns no error
	err := n.Notify(samples, hostinfo)
	checkError(t, err, "Failed to notify")
	checkCalled(t, called, 1)

	// Test that http client is still the same after next request
	httpClient := n.client
	err = n.Notify(samples, hostinfo)
	checkError(t, err, "Failed to notify")
	if httpClient != n.client {
		t.Fatalf("Expected client to be reused")
	}
	checkCalled(t, called, 2)

	// Test that http client is recreated when host info changes
	n.HostChanged()
	err = n.Notify(samples, hostinfo)
	checkError(t, err, "Failed to notify")

	if httpClient == n.client {
		t.Fatalf("Expected client to be recreated")
	}
	checkCalled(t, called, 3)
}

// Test that notify returns error when host cert is not found
func TestNotifyNoCert(t *testing.T) {
	cfg := &config.Config{
		HostCertPath:    "notfound",
		HostCertKeyPath: "notfound",
	}
	n := NewPrometheusNotifier(cfg)

	samples := createSamples()
	hostinfo := createHostInfo()

	err := n.Notify(samples, hostinfo)

	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatal("Expected error on not finding certicate")
	}
}

// Test that notify returns error when request fails
func TestNotifyRequestError(t *testing.T) {
	useInsecureTLS(t)
	_, certPath, keyPath, _ := createTestKeypair(t)

	cfg := &config.Config{
		HostCertPath:       certPath,
		HostCertKeyPath:    keyPath,
		WriteRetryAttempts: 1,
	}
	n := NewPrometheusNotifier(cfg)

	samples := createSamples()
	hostinfo := createHostInfo()
	called := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called += 1
		http.Error(w, "404 Not found", http.StatusNotFound)
	}))
	defer server.Close()
	cfg.WriteUrl = server.URL + writeUrlPath

	err := n.Notify(samples, hostinfo)
	if err == nil {
		t.Fatal("Expected error on request failure")
	}
	if called != 1 {
		t.Fatalf("Expected to call server once, got %d", called)
	}
}

// Retries & Backoff:
//   - Prometheus Remote Write compatible senders MUST retry write requests on HTTP 5xx responses
//     and MUST use a backoff algorithm to prevent overwhelming the server.
func TestRetriesAndBackoff(t *testing.T) {
	// We ignore auth and body and focus only on correct retries and backoff
	called := 0
	responseText := []string{
		"500 Internal Server Error",
		"507 Insufficient Storage",
		"",
	}
	responseCode := []int{
		http.StatusInternalServerError,
		http.StatusInsufficientStorage,
		http.StatusOK,
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, responseText[called], responseCode[called])
		called += 1
	}))
	defer server.Close()
	cfg := &config.Config{
		WriteUrl:           server.URL + writeUrlPath,
		WriteRetryAttempts: 2,
		WriteRetryMinInt:   1 * time.Millisecond,
		WriteRetryMaxInt:   2 * time.Millisecond,
	}
	client := server.Client()
	request, _ := http.NewRequest("POST", cfg.WriteUrl, nil)

	// Test that retries are done as expected and it will fail
	err := prometheusRemoteWrite(client, cfg, request)
	if err == nil {
		t.Fatal("Expected error on request failure")
	}
	checkRecoverable(t, err)
	checkCalled(t, called, int(cfg.WriteRetryAttempts))

	// Test that retries are done as expected and it will succeed
	called = 0
	cfg.WriteRetryAttempts = 3
	err = prometheusRemoteWrite(client, cfg, request)
	checkError(t, err, "Failed to send request")
}

// Retries & Backoff:
//   - They MUST NOT retry write requests on HTTP 2xx and 4xx responses other than 429.
//   - They MAY retry on HTTP 429 responses, which could result in senders "falling behind"
//     if the server cannot keep up
func TestNoRetriesOn4xx(t *testing.T) {
	// We ignore auth and body and focus only on correct retries and backoff
	called := 0
	responseText := []string{
		"400 Bad Request",
		"429 Too Many Requests",
		"404 Not Found",
		"200 OK",
	}
	responseCode := []int{
		http.StatusBadRequest,
		http.StatusTooManyRequests,
		http.StatusNotFound,
		http.StatusOK,
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, responseText[called], responseCode[called])
		called += 1
	}))
	defer server.Close()
	cfg := &config.Config{
		WriteUrl:           server.URL + writeUrlPath,
		WriteRetryAttempts: 3,
		WriteRetryMinInt:   1 * time.Millisecond,
		WriteRetryMaxInt:   2 * time.Millisecond,
	}
	client := server.Client()
	request, _ := http.NewRequest("POST", cfg.WriteUrl, nil)

	// Test that retries are done as expected and it will fail
	err := prometheusRemoteWrite(client, cfg, request)
	checkExpectedErrorContains(t, err, "http Error: 400")
	checkNonRecoverable(t, err)
	checkCalled(t, called, 1)

	// Test that retries are done on 429 but not on subsequest 404
	err = prometheusRemoteWrite(client, cfg, request)
	checkExpectedErrorContains(t, err, "http Error: 404")
	checkNonRecoverable(t, err)
	checkCalled(t, called, 1+2)

	// Last request is 200 and that should succeed without retries
	err = prometheusRemoteWrite(client, cfg, request)
	checkError(t, err, "failed to send request")
	checkCalled(t, called, 1+2+1)
}

// Test that label rules of Prometheus remote write spec are followed
func TestLabels(t *testing.T) {
	samples := createSamples()

	// With full host info
	hi := createHostInfo()
	createRequestAndCheckLabels(t, samples, hi)
	writeRequest := hostInfo2WriteRequest(hi, samples)
	checkLabelsPresence(t, writeRequest.Timeseries[0].Labels, []string{
		"__name__",
		"_id",
		"billing_marketplace",
		"billing_marketplace_account",
		"billing_marketplace_instance_id",
		"billing_model",
		"conversions_success",
		"display_name",
		"external_organization",
		"product",
		"socket_count",
		"support",
		"usage",
	})

	// With host info that is missing some values
	hi.Billing.MarketplaceAccount = ""
	createRequestAndCheckLabels(t, samples, hi)

	hi.Billing = hostinfo.BillingInfo{}
	createRequestAndCheckLabels(t, samples, hi)

	hi.SocketCount = ""
	hi.Usage = ""
	createRequestAndCheckLabels(t, samples, hi)

	hi.HostId = ""
	hi.Product = []string{}
	hi.Support = ""
	createRequestAndCheckLabels(t, samples, hi)

	hi.ConversionsSuccess = ""
	createRequestAndCheckLabels(t, samples, hi)
}

func createRequestAndCheckLabels(t *testing.T, samples []prompb.Sample, hostinfo *hostinfo.HostInfo) {
	writeRequest := hostInfo2WriteRequest(hostinfo, samples)
	for _, ts := range writeRequest.Timeseries {
		checkLabels(t, ts.Labels)
	}
}

// Helper checks

// Check that the request has headers as expected in Prometheus remote write spec
func checkPromethuesRemoteWriteHeaders(t *testing.T, r *http.Request) {
	if r.Header.Get("Content-Encoding") != "snappy" {
		t.Errorf(
			"Expected: `Content-Encoding: snappy` header, got: `%s`",
			r.Header.Get("Content-Encoding"))
	}
	if r.Header.Get("Content-Type") != "application/x-protobuf" {
		t.Errorf(
			"Expected: `Content-Type: application/x-protobuf` header, got: `%s`",
			r.Header.Get("Content-Type"))
	}
	if r.Header.Get("User-Agent") == "" {
		t.Errorf("Expected: `User-Agent` header to be set")
	}
	if r.Header.Get("X-Prometheus-Remote-Write-Version") != "0.1.0" {
		t.Errorf(
			"Expected: `X-Prometheus-Remote-Write-Version: 0.1.0` header, got: `%s`",
			r.Header.Get("X-Prometheus-Remote-Write-Version"))
	}
}

// Check that the server was called expected number of times
func checkCalled(t *testing.T, called int, expected int) {
	if called != expected {
		t.Fatalf("Expected to call server %d times, got %d", expected, called)
	}
}

// Check that the request uses host certificate for mTLS
func checkRequestUsesHostCert(t *testing.T, r *http.Request) {
	if len(r.TLS.PeerCertificates) != 1 {
		t.Errorf("Expected 1 peer certificate, got %d", len(r.TLS.PeerCertificates))
	}
	err := r.TLS.PeerCertificates[0].VerifyHostname(testHostname)
	if err != nil {
		t.Errorf("Failed to verify hostname %s", err)
	}
}

// Check that the body is Snappy compressed protobuf message
func checkRequestBody(t *testing.T, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	decoded, err := snappy.Decode(nil, body)
	if err != nil {
		t.Errorf("Failed to decode body with snappy %s", err)
	}

	writeRequest := &prompb.WriteRequest{}
	err = writeRequest.Unmarshal(decoded)
	if err != nil {
		t.Errorf("Failed to unmarshal as protobuf message %s", err)
	}
}

// Check that the error message contains expected string
func checkExpectedErrorContains(t *testing.T, err error, message string) {
	if err == nil {
		t.Fatalf("expected error with message: %s", message)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("unexpected error message: '%s' != '%s'", err.Error(), message)
	}
}

func checkRecoverable(t *testing.T, err error) {
	t.Helper()
	var notifyError *NotifyError
	if errors.As(err, &notifyError) && !notifyError.Recoverable() {
		t.Fatalf("Expected error to be recoverable. Got: %s", err.Error())
	}
}

func checkNonRecoverable(t *testing.T, err error) {
	t.Helper()
	var notifyError *NotifyError
	if errors.As(err, &notifyError) && notifyError.Recoverable() {
		t.Fatalf("Expected error to be non-recoverable. Got: %s", err.Error())
	}
}

// Check that labels follow:
//   - SHOULD contain a __name__ label.
//   - MUST NOT contain repeated label names.
//   - MUST have label names sorted in lexicographical order.
//   - MUST NOT contain any empty label names or values.
func checkLabels(t *testing.T, labels []prompb.Label) {
	// Test that __name__ label is present
	present := false
	for _, label := range labels {
		if label.Name == "__name__" {
			present = true
		}
	}
	if !present {
		t.Fatalf("Expected __name__ label to be present")
	}

	// Test that labels are sorted
	for i := 1; i < len(labels); i++ {
		if labels[i-1].Name > labels[i].Name {
			t.Fatalf("Expected labels to be sorted, got: %s > %s", labels[i-1].Name, labels[i].Name)
		}
	}

	// Test that labels are unique
	for i := 1; i < len(labels); i++ {
		if labels[i-1].Name == labels[i].Name {
			t.Fatalf("Expected labels to be unique, got: %s == %s", labels[i-1].Name, labels[i].Name)
		}
	}

	// Test that labels are not empty
	for _, label := range labels {
		if label.Name == "" || label.Value == "" {
			t.Fatalf("Expected labels to be non-empty, got: %s == %s", label.Name, label.Value)
		}
	}
}

// Check that labels with expected names are present
func checkLabelsPresence(t *testing.T, labels []prompb.Label, expected_names []string) {
	t.Helper()
	// Test that labels are present
	for _, name := range expected_names {
		present := false
		for _, label := range labels {
			if label.Name == name {
				present = true
				break
			}
		}
		if !present {
			t.Fatalf("Expected %s label to be present", name)
		}
	}
}

// Data init functions

// Test uses tlsInsecureSkipVerify = true, e.g. for mock server with self-signed certificate
func useInsecureTLS(t *testing.T) {
	tlsInsecureSkipVerify = true
	t.Cleanup(func() {
		tlsInsecureSkipVerify = false
	})
}

// Some Samples ordered by timestamp
func createSamples() []prompb.Sample {
	return []prompb.Sample{
		{Value: 1, Timestamp: time.Now().UnixMilli()},
		{Value: 2, Timestamp: time.Now().UnixMilli() + 1},
	}
}

// Dummy host info with all fields filled
func createHostInfo() *hostinfo.HostInfo {
	return &hostinfo.HostInfo{
		CpuCount:             1,
		HostId:               "test",
		HostName:             testHostname,
		SocketCount:          "1",
		Product:              []string{"123", "456"},
		Support:              "test support",
		Usage:                "test usage",
		ConversionsSuccess:   "true",
		ExternalOrganization: "test external organization",
		Billing: hostinfo.BillingInfo{
			Model:                 "test model",
			Marketplace:           "test marketplace",
			MarketplaceAccount:    "test marketplace account",
			MarketplaceInstanceId: "test marketplace instance id",
		},
	}
}

// Create a self-signed certificate for testing
//   - save the cert and key to files
//   - returns the cert, cert path and key path
func createTestKeypair(t *testing.T) (cert tls.Certificate, certPath string, keyPath string, err error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key for cert %s", err)
	}

	// Create a new certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   testHostname,
			Organization: []string{"Example Organization"},
		},
		DNSNames: []string{
			testHostname,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDataEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Create a new self-signed certificate
	derBytes, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed create cert cert %s", err)
	}

	// Encode the private key and certificate to PEM format
	keyBytes := pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certBytes := pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	// Write the private key and certificate to separate files
	dir := t.TempDir()

	keyPath = dir + "/test.key"
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("Failed to create key file %s", err)
	}
	defer keyFile.Close()
	keyFile.Write(keyBytes)

	certPath = dir + "/test.crt"
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("Failed to create cert file %s", err)
	}
	defer certFile.Close()
	certFile.Write(certBytes)

	cert, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		t.Fatalf("Failed to create cert %s", err)
	}
	return
}
