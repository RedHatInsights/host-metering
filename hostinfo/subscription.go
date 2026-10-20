package hostinfo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/RedHatInsights/host-metering/logger"
)

// Local syspurpose store, maintained by rhsmcertd and served verbatim by the
// RHSM D-Bus API (com.redhat.RHSM1.Syspurpose.GetSyspurpose). A pure local
// read with a stable JSON shape since RHEL 8.4. Overridden by tests.
var syspurposeFilePath = "/etc/rhsm/syspurpose/syspurpose.json"

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

// GetSubManIdentity returns the consumer identity reported by
// subscription-manager. On an unregistered system the command fails and
// produces no parseable output, leaving HostId and ExternalOrganization
// empty - which is what the notify policy relies on to keep unregistered
// hosts from sending data. Do not "fix" the ignored error without
// preserving that behavior.
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

// GetUsage returns the intended usage of the system (syspurpose "usage"
// field).
func GetUsage() (string, error) {
	return GetSyspurposeField("usage")
}

// GetServiceLevel returns the service level agreement (syspurpose
// "service_level_agreement" field).
func GetServiceLevel() (string, error) {
	return GetSyspurposeField("service_level_agreement")
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

	// Force English output for predictable key lookups across all locales.
	// Replace inherited LANG/LC_ALL entries instead of appending, so
	// children reading either the first or last occurrence see the forced
	// values.
	env := make([]string, 0, len(cmd.Environ())+2)
	for _, e := range cmd.Environ() {
		if strings.HasPrefix(e, "LANG=") || strings.HasPrefix(e, "LC_ALL=") {
			continue
		}
		env = append(env, e)
	}
	cmd.Env = append(env, "LANG=C.UTF-8", "LC_ALL=C.UTF-8")
	logger.Debugf("Executing `%s %s`...\n", name, strings.Join(arg, " "))

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()

	if err != nil {
		err = fmt.Errorf("`%s %s` has failed: %w", name, strings.Join(arg, " "), err)
		logger.Debugf("Stdout: %s\n", strings.TrimSpace(stdout.String()))
		logger.Debugf("Stderr: %s\n", strings.TrimSpace(stderr.String()))
		logger.Debugf("Error executing command: %s", err.Error())
		return "", err
	}

	return stdout.String(), nil
}

// subManBinaryPath is the absolute path of the subscription-manager binary.
// The binary lives in /usr/sbin on all supported majors; the absolute path
// sidesteps the /usr/bin/consolehelper wrapper (RHEL 8/9) and works under
// minimal PATHs (RHEL 10 has no /usr/bin/subscription-manager at all).
// Overridden by the unit tests, which exercise the mock from ./mocks via PATH.
var subManBinaryPath = "/usr/sbin/subscription-manager"

// execSubManCommand runs subscription-manager via its absolute path.
func execSubManCommand(command ...string) (string, error) {
	return execCommand(subManBinaryPath, command...)
}

// getKeysToTry returns the JSON key aliases for a syspurpose field, matched
// case-insensitively by parseSyspurposeJSON. The local store uses snake_case
// keys (service_level_agreement); other spellings appear in remote or cached
// forms of the data.
func getKeysToTry(fieldName string) []string {
	fn := strings.ToLower(fieldName)
	if fn == "usage" {
		return []string{"usage", "current usage", "current_usage"}
	}
	if fn == "role" {
		return []string{"role", "current role", "current_role"}
	}
	if fn == "service_level_agreement" || fn == "service-level" || fn == "servicelevel" {
		return []string{
			"service_level_agreement", "servicelevelagreement",
			"service level agreement", "service-level", "service_level",
			"servicelevel", "current service level", "current service-level",
			"current_service_level", "sla",
		}
	}
	return []string{fieldName}
}

// parseSyspurposeJSON extracts a syspurpose field from a JSON document,
// trying the field's key aliases case-insensitively. Array values are joined
// with commas. Only the leading JSON value is decoded, because
// subscription-manager can append warnings after the payload on stdout.
func parseSyspurposeJSON(data []byte, fieldName string) string {
	var jsonMap map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&jsonMap); err != nil {
		return ""
	}

	lowerJsonMap := make(map[string]interface{}, len(jsonMap))
	for k, v := range jsonMap {
		lowerJsonMap[strings.ToLower(k)] = v
	}

	for _, key := range getKeysToTry(fieldName) {
		val, ok := lowerJsonMap[strings.ToLower(key)]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			return strings.TrimSpace(v)
		case []interface{}:
			var elements []string
			for _, elem := range v {
				if elem == nil {
					continue
				}
				elements = append(elements, strings.TrimSpace(fmt.Sprintf("%v", elem)))
			}
			return strings.Join(elements, ",")
		case bool, float64:
			return strings.TrimSpace(fmt.Sprintf("%v", val))
		default:
			// Nested objects and other shapes do not produce a usable
			// label value.
			return ""
		}
	}
	return ""
}

// GetSyspurposeField reads a syspurpose field (e.g. "usage" or
// "service_level_agreement") through JSON surfaces that are invariant across
// RHEL 8.10/9/10:
//
//  1. the local syspurpose store (/etc/rhsm/syspurpose/syspurpose.json),
//     kept fresh by rhsmcertd - a pure local read serving the same data as
//     the RHSM D-Bus API;
//  2. `subscription-manager syspurpose --show` (the command insights-core
//     collects), as a fallback when the file is missing or the field is
//     empty. On registered systems this can trigger a syspurpose sync.
//
// Deprecated text outputs are deliberately not used: the top-level
// service-level/usage commands were removed in sub-man 1.30 (RHEL 10), the
// standalone syspurpose tool only ever shipped on RHEL 8, and their labels
// vary with sub-man and Candlepin server capabilities.
func GetSyspurposeField(fieldName string) (string, error) {
	if data, err := os.ReadFile(syspurposeFilePath); err == nil {
		if val := parseSyspurposeJSON(data, fieldName); val != "" {
			return val, nil
		}
	}

	if output, err := execSubManCommand("syspurpose", "--show"); err == nil {
		if val := parseSyspurposeJSON([]byte(output), fieldName); val != "" {
			return val, nil
		}
	}

	logger.Debugf("syspurpose field `%s` is not set", fieldName)
	return "", nil
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
