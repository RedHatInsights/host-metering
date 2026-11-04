package hostinfo

import "testing"

func TestGetCPUCount(t *testing.T) {
	cpuCount, err := GetCPUCount()
	if err != nil {
		t.Fatalf("failed to get the CPU count: %v", err)
	}

	if cpuCount < 1 {
		t.Fatalf("invalid value of CPU count: %d < 1", cpuCount)
	}
}

// No fallback source may answer when procfs is unusable.
func TestGetCPUCountFailsWithoutProcCPUInfo(t *testing.T) {
	restore := procMountPath
	procMountPath = t.TempDir() // directory exists, cpuinfo does not
	t.Cleanup(func() { procMountPath = restore })

	if _, err := GetCPUCount(); err == nil {
		t.Fatal("GetCPUCount succeeded without /proc/cpuinfo; no fallback may answer")
	}
}
