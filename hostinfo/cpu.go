package hostinfo

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"
)

// sysfsCPUOnlinePath is the kernel's authoritative online CPU mask.
// Overridden by tests.
var sysfsCPUOnlinePath = "/sys/devices/system/cpu/online"

// GetCPUCount returns the number of online logical CPUs (vCPUs).
//
// The count is re-read on every call so that VM resizing (CPU hotplug) and
// core offlining are reflected in subsequent samples; RefreshCpuCount in
// the collect loop depends on this.
//
// /proc/cpuinfo is the primary source: it lists exactly the online CPUs,
// is hotplug-aware on every read, shows host CPUs even when the process
// runs with a restricted CPU affinity, and matches the definition of the
// reported metric. The sysfs online mask serves the same semantics as a
// fallback. runtime.NumCPU() is deliberately not used: it reports the
// process's affinity mask as snapshotted at startup, so it is wrong under
// affinity restrictions (e.g. inside containers) and cannot observe CPU
// allocation changes after startup.
func GetCPUCount() (uint, error) {
	fs, err := procfs.NewFS("/proc")
	if err == nil {
		info, err := fs.CPUInfo()
		if err == nil && len(info) > 0 {
			return uint(len(info)), nil
		}
	}

	if count, err := readSysfsOnlineCPUCount(); err == nil {
		return count, nil
	}

	return 0, fmt.Errorf("GetCPUCount: failed to determine CPU count: /proc/cpuinfo and %s unavailable", sysfsCPUOnlinePath)
}

// readSysfsOnlineCPUCount reads and parses the sysfs online CPU mask.
func readSysfsOnlineCPUCount() (uint, error) {
	data, err := os.ReadFile(sysfsCPUOnlinePath)
	if err != nil {
		return 0, err
	}
	return parseOnlineCPUMask(string(data))
}

// parseOnlineCPUMask parses a CPU list such as "0-2,4,7" or "0" and
// returns the number of CPUs it covers.
func parseOnlineCPUMask(mask string) (uint, error) {
	mask = strings.TrimSpace(mask)
	if mask == "" {
		return 0, fmt.Errorf("empty CPU mask")
	}

	count := 0
	for _, part := range strings.Split(mask, ",") {
		if part == "" {
			continue
		}
		if low, high, found := strings.Cut(part, "-"); found {
			lo, errLo := strconv.Atoi(low)
			hi, errHi := strconv.Atoi(high)
			if errLo != nil || errHi != nil || lo > hi {
				return 0, fmt.Errorf("invalid CPU range %q", part)
			}
			count += hi - lo + 1
		} else {
			if _, err := strconv.Atoi(part); err != nil {
				return 0, fmt.Errorf("invalid CPU %q", part)
			}
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("CPU mask %q covers no CPUs", mask)
	}
	return uint(count), nil
}
