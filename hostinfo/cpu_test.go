package hostinfo

import (
	"testing"
)

func TestGetCPUCount(t *testing.T) {
	cpuCount, err := GetCPUCount()

	if err != nil {
		t.Fatalf("failed to get the CPU count: %v", err)
	}

	if cpuCount < 1 {
		t.Fatalf("invalid value of CPU count: %d < 1", cpuCount)
	}
}
