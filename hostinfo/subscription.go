package hostinfo

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"redhat.com/milton/logger"
)

func LoadSubManInformation(hi *HostInfo) {

	hostId, err := GetHostId()
	if err != nil {
		logger.Warnf("Error getting host id: %s\n", err.Error())
	} else {
		hi.HostId = hostId
	}

	logger.Debugln("Getting`subscription-manager usage`")
	usage, err := GetUsage()
	if err != nil {
		logger.Warnf("Error getting host usage: %s\n", err.Error())
	} else {
		hi.Usage = usage
	}

	logger.Debugln("Getting`subscription-manager service-level`")
	serviceLevel, err := GetServiceLevel()
	if err != nil {
		logger.Warnf("Error getting service level: %s\n", err.Error())
	} else {
		hi.Support = serviceLevel
	}

	logger.Debugln("Getting`subscription-manager facts`")
	facts, err := GetSubManFacts()
	if err != nil {
		logger.Warnf("Error getting host facts: %s\n", err.Error())
	} else {
		FactsToHostInfo(facts, hi)
	}
}

func GetHostId() (string, error) {
	output, _ := execSubManCommand("identity")
	values := parseSubManOutput(output)
	return values.get("system identity")
}

func GetUsage() (string, error) {
	output, _ := execSubManCommand("usage")
	values := parseSubManOutput(output)
	return values.get("Current Usage")
}

func GetServiceLevel() (string, error) {
	output, _ := execSubManCommand("service-level")
	values := parseSubManOutput(output)
	return values.get("Current service level")
}

func GetSubManFacts() (SubManValues, error) {
	output, _ := execSubManCommand("facts")
	return parseSubManOutput(output), nil
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
		hi.Billing.Marketplace = "aws"
	}
	if v, ok := facts["aws_account_id"]; ok {
		hi.Billing.MarketplaceAccount = v
	}
	if v, ok := facts["aws_instance_id"]; ok {
		hi.Billing.MarketplaceInstanceId = v
	}

	// Azure
	if _, ok := facts["azure_instance_id"]; ok {
		hi.Billing.Marketplace = "azure"
	}
	if v, ok := facts["azure_subscription_id"]; ok {
		hi.Billing.MarketplaceAccount = v
	}
	if v, ok := facts["azure_instance_id"]; ok {
		hi.Billing.MarketplaceInstanceId = v
	}

	// GCP
	if v, ok := facts["gcp_instance_id"]; ok {
		hi.Billing.Marketplace = v
	}
	if v, ok := facts["gcp_project_number"]; ok {
		hi.Billing.MarketplaceAccount = v
	}
	if v, ok := facts["gcp_instance_id"]; ok {
		hi.Billing.MarketplaceInstanceId = v
	}

	if hi.Billing.Marketplace != "" {
		hi.Billing.Model = "marketplace"
	}
}

func execSubManCommand(command string) (string, error) {
	cmd := exec.Command("subscription-manager", command)
	logger.Debugf("Executing `subscription-manager %s`...\n", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	if err != nil {
		err = fmt.Errorf("`subscription-manager %s` has failed: %s", command, err.Error())
		logger.Debugf("Stderr: %s\n", strings.TrimSpace(stderr.String()))
		logger.Errorf("Error executing subscription manager: %s", err.Error())
		return "", err
	}

	return stdout.String(), nil
}

func parseSubManOutput(output string) SubManValues {
	values := SubManValues{}
	reader := strings.NewReader(output)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, ":", 2)

		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Unify the letter case of keys.
		values[strings.ToLower(key)] = value
	}

	return values
}

type SubManValues map[string]string

func (values SubManValues) get(name string) (string, error) {
	v, ok := values[strings.ToLower(name)]

	if !ok {
		err := fmt.Errorf("`%s` not found", name)
		logger.Errorf("Error getting subscription info: %s", err.Error())
		return "", err
	}

	return v, nil
}
