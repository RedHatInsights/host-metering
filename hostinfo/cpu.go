package hostinfo

import (
	"fmt"

	"github.com/prometheus/procfs"
)

// procMountPath is the procfs mount point. Overridden by tests.
var procMountPath = "/proc"

// GetCPUCount returns the number of online logical CPUs (vCPUs) from /proc/cpuinfo.
func GetCPUCount() (uint, error) {
	fs, err := procfs.NewFS(procMountPath)
	if err != nil {
		return 0, fmt.Errorf("GetCPUCount: failed to open procfs: %w", err)
	}

	info, err := fs.CPUInfo()
	if err != nil {
		return 0, fmt.Errorf("GetCPUCount: failed to load CPUInfo: %w", err)
	}

	return uint(len(info)), nil
}
