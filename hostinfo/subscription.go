package hostinfo

import (
	"crypto/x509"
	"encoding/pem"
	"os"

	"redhat.com/milton/config"
)

///etc/insights-client/machine-id

// subscription-manager is using CN part of Subject field of the certificate as ConsumerId
// https://github.com/candlepin/subscription-manager/blob/main/src/subscription_manager/identity.py#L84
func GetHostId(c *config.Config) (string, error) {
	cert, err := LoadCertificate(c.HostCertPath)
	if err != nil {
		return "", err
	}

	return cert.Subject.CommonName, nil
}

func LoadCertificate(certPath string) (*x509.Certificate, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certBytes)
	return x509.ParseCertificate(block.Bytes)
}
