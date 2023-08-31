package hostinfo

import (
	"fmt"
	"strings"

	"redhat.com/milton/config"
)

type HostInfo struct {
	CpuCount                     uint
	HostId                       string
	SocketCount                  string
	Product                      string
	Support                      string
	Usage                        string
	BillingModel                 string
	BillingMarketplace           string
	BillingMarketplaceAccount    string
	BillingMarketplaceInstanceId string
}

func LoadHostInfo(c *config.Config) (*HostInfo, error) {

	cpuCount, err := GetCPUCount()
	if err != nil {
		return nil, err
	}

	hi := &HostInfo{
		CpuCount: cpuCount,
	}

	LoadSubManInformation(c, hi)

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
			fmt.Sprintf("  BillingModel: %s", hi.BillingModel),
			fmt.Sprintf("  BillingMarketplace: %s", hi.BillingMarketplace),
			fmt.Sprintf("  BillingMarketplaceAccount: %s", hi.BillingMarketplaceAccount),
			fmt.Sprintf("  BillingMarketplaceInstanceId: %s", hi.BillingMarketplaceInstanceId),
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
