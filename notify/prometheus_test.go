package notify

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
)

const (
	writeUrlPath = "/api/v1/write"
	testHostname = "host.example.test"
)

// Test happy path of prometheus notifier
func TestNotify(t *testing.T) {
	// Test are using mock server with self-signed certificate
	tlsInsecureSkipVerify = true

	// Initialize notifier and data
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

	// Test that http client is recreated when hostId changes
	hostinfo.HostId = "test2"
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
		WriteUrl:            server.URL + writeUrlPath,
		WriteRetryAttempts:  2,
		WriteRetryMinIntSec: 1,
		WriteRetryMaxIntSec: 2,
	}
	client := server.Client()
	request, _ := http.NewRequest("POST", cfg.WriteUrl, nil)

	// Test that retries are done as expected and it will fail
	err := prometheusRemoteWrite(client, cfg, request)
	if err == nil {
		t.Fatal("Expected error on request failure")
	}
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
		WriteUrl:            server.URL + writeUrlPath,
		WriteRetryAttempts:  3,
		WriteRetryMinIntSec: 1,
		WriteRetryMaxIntSec: 2,
	}
	client := server.Client()
	request, _ := http.NewRequest("POST", cfg.WriteUrl, nil)

	// Test that retries are done as expected and it will fail
	err := prometheusRemoteWrite(client, cfg, request)
	checkExpectedErrorContains(t, err, "Http Error: 400")
	checkCalled(t, called, 1)

	// Test that retries are done on 429 but not on subsequest 404
	err = prometheusRemoteWrite(client, cfg, request)
	checkExpectedErrorContains(t, err, "Http Error: 404")
	checkCalled(t, called, 1+2)

	// Last request is 200 and that should succeed without retries
	err = prometheusRemoteWrite(client, cfg, request)
	checkError(t, err, "Failed to send request")
	checkCalled(t, called, 1+2+1)
}

// Test that label rules of Prometheus remote write spec are followed
func TestLabels(t *testing.T) {
	samples := createSamples()

	// With full host info
	hi := createHostInfo()
	createRequestAndCheckLabels(t, samples, hi)

	// With host info that is missing some values
	hi.Billing.MarketplaceAccount = ""
	createRequestAndCheckLabels(t, samples, hi)

	hi.Billing = hostinfo.BillingInfo{}
	createRequestAndCheckLabels(t, samples, hi)

	hi.SocketCount = ""
	hi.Usage = ""
	createRequestAndCheckLabels(t, samples, hi)

	hi.HostId = ""
	hi.Product = ""
	hi.Support = ""
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

// Data init functions

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
		CpuCount:    1,
		HostId:      "test",
		SocketCount: "1",
		Product:     "test product",
		Support:     "test support",
		Usage:       "test usage",
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
