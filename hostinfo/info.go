package hostinfo

import (
	"fmt"

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

func (hi *HostInfo) Print() {
	fmt.Println("HostInfo:")
	fmt.Println("  CpuCount: ", hi.CpuCount)
	fmt.Println("  HostId: ", hi.HostId)
	fmt.Println("  SocketCount: ", hi.SocketCount)
	fmt.Println("  Product: ", hi.Product)
	fmt.Println("  Support: ", hi.Support)
	fmt.Println("  Usage: ", hi.Usage)
	fmt.Println("  BillingModel: ", hi.BillingModel)
	fmt.Println("  BillingMarketplace: ", hi.BillingMarketplace)
	fmt.Println("  BillingMarketplaceAccount: ", hi.BillingMarketplaceAccount)
	fmt.Println("  BillingMarketplaceInstanceId: ", hi.BillingMarketplaceInstanceId)
}
