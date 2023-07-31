package hostinfo

import (
	"fmt"

	"redhat.com/milton/config"
)

type HostInfo struct {
	CpuCount    uint
	HostId      string
	CertPath    string
	CertKeyPath string
}

func LoadHostInfo(c *config.Config) (*HostInfo, error) {

	cpuCount, err := GetCPUCount()
	if err != nil {
		return nil, err
	}

	hostId, err := GetHostId(c)
	if err != nil {
		return nil, err
	}

	return &HostInfo{
		CpuCount:    cpuCount,
		HostId:      hostId,
		CertPath:    GetCertPath(c),
		CertKeyPath: GetCertKeyPath(c),
	}, nil
}

func (hi *HostInfo) Print() {
	fmt.Println("HostInfo:")
	fmt.Println("  CpuCount: ", hi.CpuCount)
	fmt.Println("  HostId: ", hi.HostId)
	fmt.Println("  CertPath: ", hi.CertPath)
	fmt.Println("  CertKeyPath: ", hi.CertKeyPath)
}
