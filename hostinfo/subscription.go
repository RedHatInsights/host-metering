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
	hi.HostId, _ = GetHostId()
	hi.Usage, _ = GetUsage()
	hi.Support, _ = GetServiceLevel()

	facts, _ := GetSubManFacts()
	hi.SocketCount, _ = GetSocketCount(facts)
	hi.Product, _ = GetProduct(facts)
	hi.Billing, _ = GetBillingInfo(facts)
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

func GetSocketCount(facts SubManValues) (string, error) {
	return facts.get("cpu.cpu_socket(s)")
}

func GetProduct(facts SubManValues) (string, error) {
	return facts.get("distribution.name")
}

func GetBillingInfo(facts SubManValues) (BillingInfo, error) {
	bi := BillingInfo{
		Model: "marketplace",
	}

	if facts.has("aws_instance_id") {
		bi.Marketplace = "aws"
		bi.MarketplaceAccount, _ = facts.get("aws_account_id")
		bi.MarketplaceInstanceId, _ = facts.get("aws_instance_id")
		return bi, nil
	}

	if facts.has("azure_instance_id") {
		bi.Marketplace = "azure"
		bi.MarketplaceAccount, _ = facts.get("azure_subscription_id")
		bi.MarketplaceInstanceId, _ = facts.get("azure_instance_id")
		return bi, nil
	}

	if facts.has("gcp_instance_id") {
		bi.Marketplace = "gcp"
		bi.MarketplaceAccount, _ = facts.get("gcp_project_number")
		bi.MarketplaceInstanceId, _ = facts.get("gcp_instance_id")
		return bi, nil
	}

	err := fmt.Errorf("unsupported or missing marketplace values")
	logger.Errorf("Error getting billing info: %s", err.Error())
	return BillingInfo{}, err
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

func (values SubManValues) has(name string) bool {
	_, ok := values[strings.ToLower(name)]
	return ok
}

func (values SubManValues) get(name string) (string, error) {
	v, ok := values[strings.ToLower(name)]

	if !ok {
		err := fmt.Errorf("`%s` not found", name)
		logger.Errorf("Error getting subscription info: %s", err.Error())
		return "", err
	}

	return v, nil
}
