package hostinfo

import (
	"reflect"
	"testing"
)

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
	// WARNING: This function requires ./test/bin in the PATH environment
	// variable to run the mocked subscription manager instead of the real
	// one. The output of the mocked script can be controlled via other
	// environment variables, like CLOUD_PROVIDER.
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
	// Simulate RHEL 10 where service-level is removed / errors out
	t.Setenv("SIMULATE_RHEL10", "true")

	// 1. With syspurpose available
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": "Production", "service_level_agreement": "Premium", "usage": "Production"}`)
	hi := &HostInfo{}
	LoadSubManInformation(hi)

	if hi.Support != "Premium" {
		t.Errorf("Expected Support to fall back to syspurpose 'Premium' under RHEL 10, got: %s", hi.Support)
	}
	if hi.Usage != "Production" {
		t.Errorf("Expected Usage to fall back to syspurpose 'Production' under RHEL 10, got: %s", hi.Usage)
	}

	// 2. With syspurpose also unavailable/errors out - should fail/omit gracefully (Support and Usage are empty, no panic)
	t.Setenv("MOCK_SYSPURPOSE_UNAVAILABLE", "true")
	t.Setenv("MOCK_LEGACY_USAGE_UNAVAILABLE", "true")
	hiGraceful := &HostInfo{}
	LoadSubManInformation(hiGraceful)

	if hiGraceful.Support != "" {
		t.Errorf("Expected Support to be empty when service-level and syspurpose both fail, got: %s", hiGraceful.Support)
	}
	if hiGraceful.Usage != "" {
		t.Errorf("Expected Usage to be empty when usage and syspurpose both fail, got: %s", hiGraceful.Usage)
	}
}

func TestSyspurposeEnvironments(t *testing.T) {
	// Test legacy usage/service-level disabled, relying entirely on syspurpose
	t.Setenv("MOCK_LEGACY_USAGE_UNAVAILABLE", "true")
	t.Setenv("MOCK_SERVICE_LEVEL_UNAVAILABLE", "true")

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

	// Case C: Plain key-value text format (non-JSON)
	t.Setenv("MOCK_SYSPURPOSE_JSON", "Role: Production\nService Level Agreement: Premium\nUsage: Production")
	hiKV := &HostInfo{}
	LoadSubManInformation(hiKV)

	if hiKV.Support != "Premium" {
		t.Errorf("Expected Support to parse from KV: Premium, got: %s", hiKV.Support)
	}
	if hiKV.Usage != "Production" {
		t.Errorf("Expected Usage to parse from KV: Production, got: %s", hiKV.Usage)
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
	t.Setenv("MOCK_LEGACY_USAGE_UNAVAILABLE", "true")
	t.Setenv("MOCK_SERVICE_LEVEL_UNAVAILABLE", "true")

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

	// 2. JSON Null values (should fall through gracefully to empty)
	t.Setenv("MOCK_SYSPURPOSE_JSON", `{"role": null, "service_level_agreement": null, "usage": null}`)
	t.Setenv("MOCK_SYSPURPOSE_USAGE", "")
	t.Setenv("MOCK_SYSPURPOSE_ROLE", "")
	t.Setenv("MOCK_SYSPURPOSE_SLA", "")
	hiNull := &HostInfo{}
	LoadSubManInformation(hiNull)

	if hiNull.Support != "" {
		t.Errorf("Expected Support to be empty when JSON values are null, got: %s", hiNull.Support)
	}
	if hiNull.Usage != "" {
		t.Errorf("Expected Usage to be empty when JSON values are null, got: %s", hiNull.Usage)
	}

	// 3. Malformed JSON recovering via line-by-line fallback
	t.Setenv("MOCK_SYSPURPOSE_JSON", "{\n  \"role\": \"Production\", INVALID_JSON\n  Role: Production\n  Service-Level: Standard\n  Usage: Development\n}")
	hiRecover := &HostInfo{}
	LoadSubManInformation(hiRecover)

	if hiRecover.Support != "Standard" {
		t.Errorf("Expected Support to recover via line parser 'Standard', got: %s", hiRecover.Support)
	}
	if hiRecover.Usage != "Development" {
		t.Errorf("Expected Usage to recover via line parser 'Development', got: %s", hiRecover.Usage)
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
