package hostinfo

import (
	"fmt"
	"runtime"

	"github.com/prometheus/procfs"
)

// multi-platform way to get cpu core count
func GetCPUCount() (uint, error) {

	fs, err := procfs.NewFS("/proc")
	if err == nil {
		info, err := fs.CPUInfo()
		if err == nil && len(info) > 0 {
			return uint(len(info)), nil
		}
	}

	// Resilient fallback using runtime.NumCPU()
	cpuCount := uint(runtime.NumCPU())
	if cpuCount > 0 {
		return cpuCount, nil
	}

	return 0, fmt.Errorf("GetCPUCount: failed to determine CPU count")
}
