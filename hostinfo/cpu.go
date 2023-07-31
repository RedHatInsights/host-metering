package hostinfo

import (
	"fmt"

	"github.com/prometheus/procfs"
)

// multi-platform way to get cpu core count
func GetCPUCount() (uint, error) {

	fs, err := procfs.NewFS("/proc")
	if err != nil {
		return 0, fmt.Errorf("GetCPUCount: failed to open procfs: %w", err)
	}

	info, err := fs.CPUInfo()
	if err != nil {
		return 0, fmt.Errorf("GetCPUCount: failed to load CPUInfo: %w", err)
	}

	cpuCount := uint(len(info))
	return cpuCount, nil
}
