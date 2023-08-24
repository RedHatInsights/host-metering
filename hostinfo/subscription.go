package hostinfo

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

func LoadSubManInformation(cfg *config.Config, hi *HostInfo) {

	hostId, err := GetHostId(cfg)
	if err != nil {
		fmt.Println(err)
	} else {
		hi.HostId = hostId
	}

	usage, err := GetUsage()
	if err != nil {
		fmt.Println(err)
	} else {
		hi.Usage = usage
	}

	serviceLevel, err := GetServiceLevel()
	if err != nil {
		fmt.Println(err)
	} else {
		hi.Support = serviceLevel
	}

	facts, err := GetSubManFacts()
	if err != nil {
		fmt.Println(err)
	} else {
		FactsToHostInfo(facts, hi)
	}
}

func GetUsage() (string, error) {
	cmd := exec.Command("subscription-manager", "usage")

	fmt.Println("Getting`subscription-manager usage`")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error: executing `subscription-manager usage`: ", err)
		return "", err
	}
	parts := strings.SplitN(string(output), ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), nil
	}
	return "", fmt.Errorf("error parsing `subscription-manager usage` output")
}

func GetServiceLevel() (string, error) {
	cmd := exec.Command("subscription-manager", "service-level")

	fmt.Println("Getting`subscription-manager service-level`")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error: executing `subscription-manager service-level`: ", err)
		return "", err
	}
	parts := strings.SplitN(string(output), ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), nil
	}
	return "", fmt.Errorf("error parsing `subscription-manager service-level` output")
}

func GetSubManFacts() (map[string]string, error) {
	facts := make(map[string]string)

	cmd := exec.Command("subscription-manager", "facts")

	fmt.Println("Getting`subscription-manager facts`")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error: executing `subscription-manager facts`: ", err)
		return facts, err
	}
	reader := strings.NewReader(string(output))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		} else {
			// Parse key-value pairs
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				facts[key] = value
			}
		}
	}

	return facts, nil
}

func FactsToHostInfo(facts map[string]string, hi *HostInfo) {
	if v, ok := facts["cpu.cpu_socket(s)"]; ok {
		hi.SocketCount = v
	}
	if v, ok := facts["distribution.name"]; ok {
		hi.Product = v
	}

	// AWS
	if _, ok := facts["aws_instance_id"]; ok {
		hi.BillingMarketplace = "aws"
	}
	if v, ok := facts["aws_account_id"]; ok {
		hi.BillingMarketplaceAccount = v
	}
	if v, ok := facts["aws_instance_id"]; ok {
		hi.BillingMarketplaceInstanceId = v
	}

	// Azure
	if _, ok := facts["azure_instance_id"]; ok {
		hi.BillingMarketplace = "azure"
	}
	if v, ok := facts["azure_subscription_id"]; ok {
		hi.BillingMarketplaceAccount = v
	}
	if v, ok := facts["azure_instance_id"]; ok {
		hi.BillingMarketplaceInstanceId = v
	}

	// GCP
	if v, ok := facts["gcp_instance_id"]; ok {
		hi.BillingMarketplace = v
	}
	if v, ok := facts["gcp_project_number"]; ok {
		hi.BillingMarketplaceAccount = v
	}
	if v, ok := facts["gcp_instance_id"]; ok {
		hi.BillingMarketplaceInstanceId = v
	}

	if hi.BillingMarketplace != "" {
		hi.BillingModel = "marketplace"
	}
}