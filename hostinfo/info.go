package hostinfo

import (
	"fmt"
	"strings"
)

type HostInfo struct {
	CpuCount             uint
	HostId               string
	ExternalOrganization string
	SocketCount          string
	Product              string
	Support              string
	Usage                string
	Billing              BillingInfo
}

type BillingInfo struct {
	Model                 string
	Marketplace           string
	MarketplaceAccount    string
	MarketplaceInstanceId string
}

type HostInfoProvider interface {
	Load() (*HostInfo, error)
	RefreshCpuCount(*HostInfo) error
}

type SubManInfoProvider struct{}

func (smip *SubManInfoProvider) Load() (*HostInfo, error) {
	return LoadHostInfo()
}

func (smip *SubManInfoProvider) RefreshCpuCount(hi *HostInfo) error {
	return RefreshCpuCount(hi)
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
			fmt.Sprintf("  ExternalOrganization: %s", hi.ExternalOrganization),
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

func RefreshCpuCount(hi *HostInfo) error {
	cpuCount, err := GetCPUCount()
	if err != nil {
		return err
	}

	hi.CpuCount = cpuCount
	return nil
}
