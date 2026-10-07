package hostinfo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/RedHatInsights/host-metering/logger"
)

func LoadSubManInformation(hi *HostInfo) {
	identity := GetSubManIdentity()
	hi.HostId, _ = GetHostId(identity)
	hi.HostName, _ = GetHostName(identity)
	hi.ExternalOrganization, _ = GetExternalOrganization(identity)

	hi.Usage, _ = GetUsage()
	hi.Support, _ = GetServiceLevel()

	facts, _ := GetSubManFacts()
	hi.SocketCount, _ = GetSocketCount(facts)
	hi.Product, _ = GetProduct(facts)
	hi.ConversionsSuccess, _ = GetConversionsSuccess(facts)
	hi.Billing, _ = GetBillingInfo(facts)
}

func GetSubManIdentity() SubManValues {
	output, _ := execSubManCommand("identity")
	return parseSubManOutput(output)
}

func GetHostId(identity SubManValues) (string, error) {
	return identity.get("system identity")
}

func GetHostName(identity SubManValues) (string, error) {
	return identity.get("name")
}

func GetExternalOrganization(identity SubManValues) (string, error) {
	return identity.get("org ID")
}

func GetUsage() (string, error) {
	output, err := execSubManCommand("usage")
	if err == nil {
		values := parseSubManOutput(output)
		if val, err := values.get("Current Usage"); err == nil && val != "" {
			return val, nil
		}
	}

	// Graceful fallback to syspurpose usage, then role
	logger.Debugf("subscription-manager usage failed or empty, falling back to syspurpose")
	if val, err := GetSyspurposeField("usage"); err == nil && val != "" {
		return val, nil
	}
	if val, err := GetSyspurposeField("role"); err == nil && val != "" {
		return val, nil
	}

	return "", nil
}

func GetServiceLevel() (string, error) {
	output, err := execSubManCommand("service-level")
	if err == nil {
		values := parseSubManOutput(output)
		if val, err := values.get("Current service level"); err == nil && val != "" {
			return val, nil
		}
	}

	// Graceful fallback to syspurpose service_level_agreement or service-level
	logger.Debugf("subscription-manager service-level failed or empty, falling back to syspurpose SLA")
	if val, err := GetSyspurposeField("service_level_agreement"); err == nil && val != "" {
		return val, nil
	}
	if val, err := GetSyspurposeField("service-level"); err == nil && val != "" {
		return val, nil
	}

	return "", nil
}

func GetSubManFacts() (SubManValues, error) {
	output, _ := execSubManCommand("facts")
	return parseSubManOutput(output), nil
}

func GetSocketCount(facts SubManValues) (string, error) {
	return facts.get("cpu.cpu_socket(s)")
}

func GetProduct(facts SubManValues) ([]string, error) {
	output, _ := execSubManCommand("list", "--installed")
	values := parseSubManOutputMultiVal(output)
	return values.get("Product ID")
}

func GetConversionsSuccess(facts SubManValues) (string, error) {
	value, err := facts.get("conversions.success")
	if err == nil {
		value = strings.ToLower(value)
	}
	return value, err
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
	logger.Debugf("Error getting billing info: %s", err.Error())
	return BillingInfo{}, err
}

func execCommand(name string, arg ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, arg...)

	// Set LANG and LC_ALL to C.UTF-8 to force English output for predictable key lookups across all locales
	cmd.Env = append(cmd.Environ(), "LANG=C.UTF-8", "LC_ALL=C.UTF-8")
	logger.Debugf("Executing `%s %s`...\n", name, arg)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	if err != nil {
		err = fmt.Errorf("`%s %s` has failed: %w", name, arg, err)
		logger.Debugf("Stdout: %s\n", strings.TrimSpace(stdout.String()))
		logger.Debugf("Stderr: %s\n", strings.TrimSpace(stderr.String()))
		logger.Debugf("Error executing command: %s", err.Error())
		return "", err
	}

	return stdout.String(), nil
}

func execSubManCommand(command ...string) (string, error) {
	return execCommand("subscription-manager", command...)
}

func cleanCommandOutput(output string) string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "###") || strings.Contains(trimmed, "Mocked output") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func getKeysToTry(fieldName string) []string {
	fn := strings.ToLower(fieldName)
	if fn == "usage" {
		return []string{"usage", "current usage", "current_usage"}
	}
	if fn == "role" {
		return []string{"role", "current role", "current_role"}
	}
	if fn == "service_level_agreement" || fn == "service-level" || fn == "servicelevel" {
		return []string{"service_level_agreement", "service level agreement", "service-level", "servicelevel", "current service level", "current_service_level", "sla"}
	}
	return []string{fieldName}
}

func parseSyspurposeOutput(output string, fieldName string) string {
	cleaned := cleanCommandOutput(output)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}

	// Try JSON first
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(cleaned), &jsonMap); err == nil {
		keysToTry := getKeysToTry(fieldName)
		for _, key := range keysToTry {
			if val, ok := jsonMap[key]; ok {
				if val == nil {
					return ""
				}
				if strVal, ok := val.(string); ok {
					return strings.TrimSpace(strVal)
				}
				if arrVal, ok := val.([]interface{}); ok {
					var elements []string
					for _, elem := range arrVal {
						if elemStr, ok := elem.(string); ok {
							elements = append(elements, strings.TrimSpace(elemStr))
						} else if elem != nil {
							elements = append(elements, fmt.Sprintf("%v", elem))
						}
					}
					return strings.Join(elements, ",")
				}
				return strings.TrimSpace(fmt.Sprintf("%v", val))
			}
		}
		return ""
	}

	// Fallback to line-by-line key-value parsing
	reader := strings.NewReader(cleaned)
	scanner := bufio.NewScanner(reader)
	keysToTry := getKeysToTry(fieldName)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		// Strip potential JSON syntax if output was partially JSON-like but unmarshal failed
		key = strings.Trim(key, `"{}, `)
		value = strings.Trim(value, `"{}, `)

		for _, k := range keysToTry {
			if key == strings.ToLower(k) {
				return value
			}
		}
	}

	// If the output is just a single line without any colon, return it
	lines := strings.Split(cleaned, "\n")
	if len(lines) == 1 {
		singleLine := strings.TrimSpace(lines[0])
		if singleLine != "" && !strings.Contains(singleLine, ":") {
			return singleLine
		}
	}

	return ""
}

func GetSyspurposeField(fieldName string) (string, error) {
	// Try standard "syspurpose show" first
	var output string
	var err error

	output, err = execSubManCommand("syspurpose", "show")
	if err != nil {
		output, err = execCommand("syspurpose", "show")
	}

	if err == nil && output != "" {
		val := parseSyspurposeOutput(output, fieldName)
		if val != "" {
			return val, nil
		}
	}

	// Fallback/alternative: Try specific subcommands (e.g. syspurpose usage or syspurpose role)
	output, err = execSubManCommand("syspurpose", fieldName)
	if err != nil {
		output, err = execCommand("syspurpose", fieldName)
	}

	if err == nil && output != "" {
		val := parseSyspurposeOutput(output, fieldName)
		if val != "" {
			return val, nil
		}
	}

	return "", fmt.Errorf("field `%s` not found in syspurpose", fieldName)
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
		logger.Debugf("Unable to get subscription info: %s", err.Error())
		return "", err
	}

	return v, nil
}

func parseSubManOutputMultiVal(output string) SubManMultiValues {
	values := SubManMultiValues{}
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
		values.add(key, value)
	}

	return values
}

type SubManMultiValues map[string][]string

func (values SubManMultiValues) add(name string, value string) {
	key := strings.ToLower(name)
	v, ok := values[key]
	if !ok {
		values[key] = []string{value}
		return
	}

	values[key] = append(v, value)
}

func (values SubManMultiValues) get(name string) ([]string, error) {
	v, ok := values[strings.ToLower(name)]

	if !ok {
		err := fmt.Errorf("`%s` not found", name)
		logger.Warnf("Unable to get subscription info: %s", err.Error())
		return []string{}, err
	}

	return v, nil
}
