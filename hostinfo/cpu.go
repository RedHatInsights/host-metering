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

	if len(info) == 0 {
		return 0, fmt.Errorf("GetCPUCount: no value in CPUInfo")
	}

	cpuCores := info[0].CPUCores

	return cpuCores, nil
}
