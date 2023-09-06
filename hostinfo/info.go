package hostinfo

import (
	"fmt"
	"strings"
)

type HostInfo struct {
	CpuCount    uint
	HostId      string
	SocketCount string
	Product     string
	Support     string
	Usage       string
	Billing     BillingInfo
}

type BillingInfo struct {
	Model                 string
	Marketplace           string
	MarketplaceAccount    string
	MarketplaceInstanceId string
}

func LoadHostInfo() (*HostInfo, error) {
	cpuCount, err := GetCPUCount()
	if err != nil {
		return nil, err
	}

	hi := &HostInfo{
		CpuCount: cpuCount,
	}
	LoadSubManInformation(hi)

	return hi, nil
}

func (hi *HostInfo) String() string {
	return strings.Join(
		[]string{
			"HostInfo:",
			fmt.Sprintf("  CpuCount: %d", hi.CpuCount),
			fmt.Sprintf("  HostId: %s", hi.HostId),
			fmt.Sprintf("  SocketCount: %s", hi.SocketCount),
			fmt.Sprintf("  Product: %s", hi.Product),
			fmt.Sprintf("  Support: %s", hi.Support),
			fmt.Sprintf("  Usage: %s", hi.Usage),
			fmt.Sprintf("  Billing.Model: %s", hi.Billing.Model),
			fmt.Sprintf("  Billing.Marketplace: %s", hi.Billing.Marketplace),
			fmt.Sprintf("  Billing.MarketplaceAccount: %s", hi.Billing.MarketplaceAccount),
			fmt.Sprintf("  Billing.MarketplaceInstanceId: %s", hi.Billing.MarketplaceInstanceId),
		}, "\n")
}

func (hi *HostInfo) RefreshCpuCount() error {
	cpuCount, err := GetCPUCount()
	if err != nil {
		return err
	}

	hi.CpuCount = cpuCount
	return nil
}
