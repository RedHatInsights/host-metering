package notify

import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

func TestFilterSamplesByAge(t *testing.T) {
	now := time.Now().UnixMilli()
	samples := []prompb.Sample{
		{Value: 1, Timestamp: now - 10000},
		{Value: 2, Timestamp: now - 8000},
		{Value: 3, Timestamp: now - 6000},
		{Value: 4, Timestamp: now - 4000},
		{Value: 5, Timestamp: now - 2000},
		{Value: 6, Timestamp: now - 1},
	}
	// set maxAge to 5s, thus expect 3 samples
	filtered := FilterSamplesByAge(samples, 5)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(filtered))
	}
	if filtered[0].Value != 4 || filtered[1].Value != 5 || filtered[2].Value != 6 {
		t.Errorf("Expected samples with values 4, 5, 6, got %v", filtered)
	}

	// set maxAge to 11s, thus expect all samples
	filtered = FilterSamplesByAge(samples, 11)
	if len(filtered) != len(samples) {
		t.Errorf("Expected %d samples, got %d", len(samples), len(filtered))
	}

	// set maxAge to 0s, thus expect no samples
	filtered = FilterSamplesByAge(samples, 0)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 samples, got %d", len(filtered))
	}

}
