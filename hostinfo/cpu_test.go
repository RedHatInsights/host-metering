package hostinfo

import "testing"

func TestParseOnlineCPUMask(t *testing.T) {
	tests := []struct {
		name     string
		mask     string
		expected uint
		wantErr  bool
	}{
		{name: "single cpu", mask: "0", expected: 1},
		{name: "simple range", mask: "0-3", expected: 4},
		{name: "ranges and singles", mask: "0-2,4,7", expected: 5},
		{name: "whitespace padded", mask: " 0-21 \n", expected: 22},
		{name: "non-contiguous ranges", mask: "0-1,4-5,8", expected: 5},
		{name: "empty mask", mask: "", wantErr: true},
		{name: "whitespace only", mask: "   ", wantErr: true},
		{name: "garbage", mask: "zero", wantErr: true},
		{name: "inverted range", mask: "5-2", wantErr: true},
		{name: "trailing comma", mask: "0-1,", expected: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOnlineCPUMask(tt.mask)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseOnlineCPUMask(%q) = %d, expected error", tt.mask, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOnlineCPUMask(%q) failed: %v", tt.mask, err)
			}
			if got != tt.expected {
				t.Errorf("parseOnlineCPUMask(%q) = %d, want %d", tt.mask, got, tt.expected)
			}
		})
	}
}

func TestGetCPUCount(t *testing.T) {
	// Smoke test: on any Linux test environment at least one CPU is
	// online, and the count must come from a live source.
	count, err := GetCPUCount()
	if err != nil {
		t.Fatalf("GetCPUCount failed: %v", err)
	}
	if count == 0 {
		t.Fatalf("GetCPUCount returned 0")
	}

	// The sysfs fallback must agree with the primary source's semantics.
	sysfsCount, err := readSysfsOnlineCPUCount()
	if err != nil {
		t.Fatalf("readSysfsOnlineCPUCount failed: %v", err)
	}
	if count != sysfsCount {
		t.Errorf("procfs count %d differs from sysfs online count %d", count, sysfsCount)
	}
}
