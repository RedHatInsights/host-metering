package hostinfo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The tests exercise the mocked subscription-manager from ./mocks, which
	// `make test` and the spec %check section put on PATH. Fail fast if a
	// real subscription-manager would be picked up instead: the CLI tier
	// runs `syspurpose --show`, which can trigger a sync on a registered
	// host.
	if path, err := exec.LookPath("subscription-manager"); err != nil || !strings.Contains(path, "/mocks/") {
		fmt.Fprintln(os.Stderr, "tests require the mocked subscription-manager from ./mocks on PATH (run via `make test`)")
		os.Exit(1)
	}

	// Point the production absolute-path override at the PATH-resolved mock
	// (the LookPath check above proves it is reachable).
	subManBinaryPath = "subscription-manager"

	// Default the syspurpose store to a non-existent path so tests exercise
	// the CLI fallback tier deterministically (a real store could exist on a
	// developer machine); the file-tier tests below point it at fixtures.
	syspurposeFilePath = "/nonexistent-test/syspurpose/syspurpose.json"
	os.Exit(m.Run())
}

func TestLoadSubManInformation(t *testing.T) {
	// Define the expected general host info.
	expected := &HostInfo{
		HostId:               "01234567-89ab-cdef-0123-456789abcdef",
		HostName:             "host.mock.test",
		ExternalOrganization: "12345678",
		SocketCount:          "3",
		Product:              []string{"394", "69"},
		Support:              "Premium",
		Usage:                "Production",
		ConversionsSuccess:   "true",
	}

	// Test the host info for AWS.
	expected.Billing = BillingInfo{
		Model:                 "marketplace",
		Marketplace:           "aws",
		MarketplaceAccount:    "000000000000",
		MarketplaceInstanceId: "1-11111111111111111",
	}
	hostInfo := getHostInfo(t, "aws")
	compareHostInfo(t, hostInfo, expected)

	// Test the host info for Azure.
	expected.Billing = BillingInfo{
		Model:                 "marketplace",
		Marketplace:           "azure",
		MarketplaceAccount:    "00000000-0000-0000-0000-000000000000",
		MarketplaceInstanceId: "11111111-1111-1111-1111-111111111111",
	}
	hostInfo = getHostInfo(t, "azure")
	compareHostInfo(t, hostInfo, expected)

	// Test the host info for GCP.
	expected.Billing = BillingInfo{
		Model:                 "marketplace",
		Marketplace:           "gcp",
		MarketplaceAccount:    "000000000000",
		MarketplaceInstanceId: "1111111111111111111",
	}
	hostInfo = getHostInfo(t, "gcp")
	compareHostInfo(t, hostInfo, expected)
}

func getHostInfo(t *testing.T, cloudProvider string) *HostInfo {
	// WARNING: This function requires the ./mocks directory in the PATH
	// environment variable to run the mocked subscription-manager instead of
	// the real one. The output of the mocked script can be controlled via
	// other environment variables, like CLOUD_PROVIDER.
	t.Setenv("CLOUD_PROVIDER", cloudProvider)

	hostInfo := &HostInfo{}
	LoadSubManInformation(hostInfo)
	t.Log(hostInfo.String())
	return hostInfo
}

func compareHostInfo(t *testing.T, hi *HostInfo, expected *HostInfo) {
	if hi.HostId != expected.HostId {
		t.Fatalf("an unexpected value of HostId: %v", hi.HostId)
	}

	if hi.SocketCount != expected.SocketCount {
		t.Fatalf("an unexpected value of SocketCount: %v", hi.SocketCount)
	}

	if !reflect.DeepEqual(hi.Product, expected.Product) {
		t.Fatalf("an unexpected value of Product: %v", hi.Product)
	}

	if hi.Support != expected.Support {
		t.Fatalf("an unexpected value of Support: %v", hi.Support)
	}

	if hi.Usage != expected.Usage {
		t.Fatalf("an unexpected value of Usage: %v", hi.Usage)
	}

	if hi.ConversionsSuccess != expected.ConversionsSuccess {
		t.Fatalf("an unexpected value of ConversionsSuccess: %v", hi.ConversionsSuccess)
	}

	if hi.Billing.Model != expected.Billing.Model {
		t.Fatalf("an unexpected value of Model: %v", hi.Billing.Model)
	}

	if hi.Billing.Marketplace != expected.Billing.Marketplace {
		t.Fatalf("an unexpected value of Marketplace: %v", hi.Billing.Marketplace)
	}

	if hi.Billing.MarketplaceAccount != expected.Billing.MarketplaceAccount {
		t.Fatalf("an unexpected value of MarketplaceAccount: %v", hi.Billing.MarketplaceAccount)
	}

	if hi.Billing.MarketplaceInstanceId != expected.Billing.MarketplaceInstanceId {
		t.Fatalf("an unexpected value of MarketplaceInstanceId: %v", hi.Billing.MarketplaceInstanceId)
	}

}

func TestRHEL10Environment(t *testing.T) {
	// RHEL 10 removed the top-level service-level/usage commands. Usage and
	// SLA resolve through the syspurpose JSON surfaces, which are invariant
	// across RHEL 8.10/9/10, so identity/facts collection is unaffected.
	t.Setenv("SIMULATE_RHEL10", "true")

	// 1. With syspurpose available
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": "Production", "service_level_agreement": "Premium", "usage": "Production"}`)
	hi := &HostInfo{}
	LoadSubManInformation(hi)

	if hi.Support != "Premium" {
		t.Errorf("Expected Support to be 'Premium' via syspurpose under RHEL 10, got: %s", hi.Support)
	}
	if hi.Usage != "Production" {
		t.Errorf("Expected Usage to be 'Production' via syspurpose under RHEL 10, got: %s", hi.Usage)
	}

	// 2. With syspurpose also unavailable - should fail gracefully
	// (Support and Usage are empty, no panic)
	t.Setenv("MOCK_SYSPURPOSE_UNAVAILABLE", "true")
	hiGraceful := &HostInfo{}
	LoadSubManInformation(hiGraceful)

	if hiGraceful.Support != "" {
		t.Errorf("Expected Support to be empty when syspurpose is unavailable, got: %s", hiGraceful.Support)
	}
	if hiGraceful.Usage != "" {
		t.Errorf("Expected Usage to be empty when syspurpose is unavailable, got: %s", hiGraceful.Usage)
	}
}

func TestSyspurposeEnvironments(t *testing.T) {
	// Case A: Standard JSON output
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": "Development", "service_level_agreement": "Standard", "usage": "Development"}`)
	hi := &HostInfo{}
	LoadSubManInformation(hi)

	if hi.Support != "Standard" {
		t.Errorf("Expected Support to be Standard, got: %s", hi.Support)
	}
	if hi.Usage != "Development" {
		t.Errorf("Expected Usage to be Development, got: %s", hi.Usage)
	}

	// Case B: Alternate keys / structures in JSON
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"current_role": "Production", "sla": "Self-Support", "current_usage": "Production"}`)
	hiAlt := &HostInfo{}
	LoadSubManInformation(hiAlt)

	if hiAlt.Support != "Self-Support" {
		t.Errorf("Expected Support to parse 'sla': Self-Support, got: %s", hiAlt.Support)
	}
	if hiAlt.Usage != "Production" {
		t.Errorf("Expected Usage to parse 'current_usage': Production, got: %s", hiAlt.Usage)
	}
}

func TestSyspurposeFileStore(t *testing.T) {
	// The local syspurpose store is the primary, side-effect-free source.
	file := filepath.Join(t.TempDir(), "syspurpose.json")
	syspurposeFilePath = file
	defer func() {
		syspurposeFilePath = "/nonexistent-test/syspurpose/syspurpose.json"
	}()

	// The file tier wins over the CLI tier.
	if err := os.WriteFile(file, []byte(`{"service_level_agreement": "Self-Support", "usage": "Development"}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"service_level_agreement": "Premium", "usage": "Production"}`)

	if val, _ := GetServiceLevel(); val != "Self-Support" {
		t.Errorf("Expected file-tier SLA 'Self-Support', got: %q", val)
	}
	if val, _ := GetUsage(); val != "Development" {
		t.Errorf("Expected file-tier usage 'Development', got: %q", val)
	}

	// camelCase keys (remote store form) are matched case-insensitively.
	if err := os.WriteFile(file, []byte(`{"serviceLevelAgreement": "Premium", "Usage": "Production"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if val, _ := GetServiceLevel(); val != "Premium" {
		t.Errorf("Expected case-insensitive SLA 'Premium', got: %q", val)
	}

	// An empty field in the file falls through to the CLI tier.
	if err := os.WriteFile(file, []byte(`{"usage": ""}`), 0644); err != nil {
		t.Fatal(err)
	}
	if val, _ := GetUsage(); val != "Production" {
		t.Errorf("Expected CLI-tier usage 'Production' after empty file value, got: %q", val)
	}

	// Garbage (non-JSON) file content falls through to the CLI tier.
	if err := os.WriteFile(file, []byte("not json at all"), 0644); err != nil {
		t.Fatal(err)
	}
	if val, _ := GetUsage(); val != "Production" {
		t.Errorf("Expected CLI-tier usage 'Production' after garbage file content, got: %q", val)
	}

	// A missing file falls back to the CLI tier.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if val, _ := GetUsage(); val != "Production" {
		t.Errorf("Expected CLI-tier usage 'Production' with missing file, got: %q", val)
	}
}

func TestSyspurposeCLITrailingOutput(t *testing.T) {
	// subscription-manager can append warnings after the JSON payload on
	// stdout; only the leading JSON value must be decoded.
	t.Setenv("MOCK_SYSPURPOSE_JSON", "{\n  \"usage\": \"Production\"\n}\nWARNING: yum plugins were recently enabled")

	if val, _ := GetUsage(); val != "Production" {
		t.Errorf("Expected 'Production' from leading JSON value, got: %q", val)
	}
}

func TestCloudMarketplaceFacts(t *testing.T) {
	// This test directly exercises GetBillingInfo to verify cloud marketplace facts extraction
	factsAWS := SubManValues{
		"aws_account_id":  "123456789012",
		"aws_instance_id": "i-0123456789abcdef0",
	}
	biAWS, err := GetBillingInfo(factsAWS)
	if err != nil {
		t.Fatalf("Failed to parse AWS billing info: %v", err)
	}
	if biAWS.Marketplace != "aws" || biAWS.MarketplaceAccount != "123456789012" || biAWS.MarketplaceInstanceId != "i-0123456789abcdef0" {
		t.Errorf("Unexpected AWS billing info: %+v", biAWS)
	}

	factsAzure := SubManValues{
		"azure_subscription_id": "sub-id-123",
		"azure_instance_id":     "inst-id-456",
	}
	biAzure, err := GetBillingInfo(factsAzure)
	if err != nil {
		t.Fatalf("Failed to parse Azure billing info: %v", err)
	}
	if biAzure.Marketplace != "azure" || biAzure.MarketplaceAccount != "sub-id-123" || biAzure.MarketplaceInstanceId != "inst-id-456" {
		t.Errorf("Unexpected Azure billing info: %+v", biAzure)
	}

	factsGCP := SubManValues{
		"gcp_project_number": "proj-789",
		"gcp_instance_id":    "inst-999",
	}
	biGCP, err := GetBillingInfo(factsGCP)
	if err != nil {
		t.Fatalf("Failed to parse GCP billing info: %v", err)
	}
	if biGCP.Marketplace != "gcp" || biGCP.MarketplaceAccount != "proj-789" || biGCP.MarketplaceInstanceId != "inst-999" {
		t.Errorf("Unexpected GCP billing info: %+v", biGCP)
	}
}

func TestSyspurposeEdgeCases(t *testing.T) {
	// 1. JSON Array values
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": ["Production", "Backup"], "service_level_agreement": ["Premium"], "usage": ["Production"]}`)
	hiArray := &HostInfo{}
	LoadSubManInformation(hiArray)

	if hiArray.Support != "Premium" {
		t.Errorf("Expected Support to parse single-element array 'Premium', got: %s", hiArray.Support)
	}
	if hiArray.Usage != "Production" {
		t.Errorf("Expected Usage to parse array value 'Production', got: %s", hiArray.Usage)
	}

	// 2. JSON Null values (should resolve to empty, no panic)
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": null, "service_level_agreement": null, "usage": null}`)
	hiNull := &HostInfo{}
	LoadSubManInformation(hiNull)

	if hiNull.Support != "" {
		t.Errorf("Expected Support to be empty when JSON values are null, got: %s", hiNull.Support)
	}
	if hiNull.Usage != "" {
		t.Errorf("Expected Usage to be empty when JSON values are null, got: %s", hiNull.Usage)
	}

	// 3. Malformed JSON resolves to empty (no text-format recovery)
	t.Setenv("MOCK_SYSPURPOSE_JSON", "{\n  \"role\": \"Production\", INVALID_JSON\n}")
	hiMalformed := &HostInfo{}
	LoadSubManInformation(hiMalformed)

	if hiMalformed.Support != "" {
		t.Errorf("Expected Support to be empty for malformed JSON, got: %s", hiMalformed.Support)
	}
	if hiMalformed.Usage != "" {
		t.Errorf("Expected Usage to be empty for malformed JSON, got: %s", hiMalformed.Usage)
	}

	// 4. On-premise non-cloud facts
	factsOnPrem := SubManValues{
		"cpu.cpu_socket(s)": "4",
		"system.memory":     "16384",
	}
	biOnPrem, err := GetBillingInfo(factsOnPrem)
	if err == nil {
		t.Errorf("Expected error for non-cloud on-premise facts, got billing info: %+v", biOnPrem)
	}
	if biOnPrem.Marketplace != "" {
		t.Errorf("Expected empty marketplace for on-premise system, got: %s", biOnPrem.Marketplace)
	}
}

func TestSyspurposeCaseInsensitiveJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		fieldName string
		expected  string
	}{
		{
			name:      "mixed-case key ServiceLevel",
			json:      `{"ServiceLevel": "Premium"}`,
			fieldName: "service-level",
			expected:  "Premium",
		},
		{
			name:      "uppercase key ROLE",
			json:      `{"ROLE": "Production"}`,
			fieldName: "role",
			expected:  "Production",
		},
		{
			name:      "mixed-case candidate match",
			json:      `{"current_usage": "Production"}`,
			fieldName: "USAGE",
			expected:  "Production",
		},
		{
			name:      "hyphenated label key",
			json:      `{"current service-level": "Premium"}`,
			fieldName: "service_level_agreement",
			expected:  "Premium",
		},
		{
			name:      "multi-element array joined with comma",
			json:      `{"role": ["Production", "Backup"]}`,
			fieldName: "role",
			expected:  "Production,Backup",
		},
		{
			name:      "nested object is not a usable value",
			json:      `{"usage": {"unexpected": "shape"}}`,
			fieldName: "usage",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSyspurposeJSON([]byte(tt.json), tt.fieldName)
			if got != tt.expected {
				t.Errorf("parseSyspurposeJSON(%q, %q) = %q; want %q", tt.json, tt.fieldName, got, tt.expected)
			}
		})
	}
}
