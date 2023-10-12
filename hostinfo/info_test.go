package hostinfo

import (
	"testing"
)

func TestHostInfo(t *testing.T) {
	hi, err := LoadHostInfo()
	checkError(t, err, "failed to load host info")

	// Check the CPU count.
	cpuCount, err := GetCPUCount()
	checkError(t, err, "failed to get CPU count")

	if hi.CpuCount != cpuCount {
		t.Fatalf("unexpected number of CPUs: %d != %d", hi.CpuCount, cpuCount)
	}

	// Enforce the CPU count to get a predictable output.
	hi.CpuCount = 64

	// Define the expected defaults.
	expectedString := "HostInfo:\n" +
		"  CpuCount: 64\n" +
		"  HostId: 01234567-89ab-cdef-0123-456789abcdef\n" +
		"  ExternalOrganization: 12345678\n" +
		"  SocketCount: 3\n" +
		"  Product: Red Hat Enterprise Linux Server\n" +
		"  Support: Premium\n" +
		"  Usage: Production\n" +
		"  Billing.Model: marketplace\n" +
		"  Billing.Marketplace: aws\n" +
		"  Billing.MarketplaceAccount: 000000000000\n" +
		"  Billing.MarketplaceInstanceId: 1-11111111111111111"

	if hi.String() != expectedString {
		t.Fatalf("unexpected string:\n%s\n!=\n%s", hi.String(), expectedString)
	}

	// Refresh the CPU count.
	err = RefreshCpuCount(hi)
	checkError(t, err, "failed to refresh CPU count")

	if hi.CpuCount != cpuCount {
		t.Fatalf("unexpected number of CPUs: %d != %d", hi.CpuCount, cpuCount)
	}

	// Reset the CPU count. Nothing else should change.
	hi.CpuCount = 64

	if hi.String() != expectedString {
		t.Fatalf("unexpected string:\n%s\n!=\n%s", hi.String(), expectedString)
	}
}

func checkError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}
