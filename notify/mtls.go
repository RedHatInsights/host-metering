package notify

import (
	"crypto/tls"
	"net/http"
	"time"

	"redhat.com/milton/hostinfo"
)

const requestTimeout time.Duration = 60 * time.Second

// Create HTTP client with host certificate for Mutual TLS authentication
func NewMTLSHttpClient(hostInfo *hostinfo.HostInfo) *http.Client {
	cert, err := tls.LoadX509KeyPair(hostInfo.CertPath, hostInfo.CertKeyPath)
	if err != nil {
		panic(err)
	}

	// Not specifying a RootCAs field as we rely on the system's root CA store
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}
