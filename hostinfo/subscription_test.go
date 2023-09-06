package hostinfo

import (
	"testing"
)

func TestLoadSubManInformation(t *testing.T) {
	// Define the expected general host info.
	expected := &HostInfo{
		HostId:      "01234567-89ab-cdef-0123-456789abcdef",
		SocketCount: "3",
		Product:     "Red Hat Enterprise Linux Server",
		Support:     "Premium",
		Usage:       "Production",
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

	if hi.Product != expected.Product {
		t.Fatalf("an unexpected value of Product: %v", hi.Product)
	}

	if hi.Support != expected.Support {
		t.Fatalf("an unexpected value of Support: %v", hi.Support)
	}

	if hi.Usage != expected.Usage {
		t.Fatalf("an unexpected value of Usage: %v", hi.Usage)
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
