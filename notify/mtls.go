package notify

import (
	"crypto/tls"
	"net/http"
	"time"

	"redhat.com/milton/config"
)

const requestTimeout time.Duration = 60 * time.Second

// Create HTTP client with host certificate for Mutual TLS authentication
func NewMTLSHttpClient(cfg *config.Config) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.HostCertPath, cfg.HostCertKeyPath)
	if err != nil {
		return nil, err
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
	}, nil
}
