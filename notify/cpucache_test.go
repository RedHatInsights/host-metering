package notify

import (
	"testing"
)

// Test basic functionality of the CpuCache - how it would be used by milton
func TestCpuCacheBasics(t *testing.T) {
	// Create a new CpuCache instance
	dir := t.TempDir()
	logPath := dir + "/cpucache"
	cache, err := NewCpuCache(logPath)
	if err != nil {
		t.Fatalf("failed to create CpuCache: %v", err)
	}
	defer cache.Close()

	// Write some sample cpuCount data to the cache
	err = cache.Write(4)
	if err != nil {
		t.Fatalf("failed to write sample data to CpuCache: %v", err)
	}

	// Another sample
	err = cache.Write(6)
	if err != nil {
		t.Fatalf("failed to write sample data to CpuCache: %v", err)
	}

	// Get all samples from the cache
	samples, lastIndex, err := cache.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from CpuCache: %v", err)
	}

	// Verify that the last index is 2 as there should be 2 samples
	if lastIndex != 2 {
		t.Fatalf("expected last index of 2, got %d", lastIndex)
	}

	// Verify that the sample data was written correctly
	if len(samples) != 2 {
		t.Fatalf("expected 2 sample, got %d", len(samples))
	}
	if samples[0].Value != 4 {
		t.Fatalf("expected sample value of 4, got %f", samples[0].Value)
	}
	if samples[1].Value != 6 {
		t.Fatalf("expected sample value of 6, got %f", samples[1].Value)
	}

	// Truncate the cache to the last item read
	err = cache.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate CpuCache: %v", err)
	}

	// Get all samples from the cache again, to check that the truncation worked
	samples, lastIndex, err = cache.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from CpuCache: %v", err)
	}

	// Verify that the cache is now empty
	if len(samples) != 0 {
		t.Fatalf("expected 0 samples, got %d", len(samples))
	}
	// Last index should still the same
	if lastIndex != 2 {
		t.Fatalf("expected last index of 2, got %d", lastIndex)
	}

	// Try again, but this time there is nothing to truncate
	err = cache.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate CpuCache when there is nothing to truncate: %v", err)
	}

	// Simulate next iteration: write sample, obtain it and truncate
	err = cache.Write(8)
	if err != nil {
		t.Fatalf("failed to write sample data to CpuCache: %v", err)
	}
	samples, lastIndex, err = cache.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from CpuCache: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].Value != 8 {
		t.Fatalf("expected sample value of 8, got %f", samples[0].Value)
	}
	err = cache.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate CpuCache: %v", err)
	}

}

// Test scenario where Prometheus server is not intially reachable (cache is not
// truncated). And milton is restarted in the mean time.
func TestRestart(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/cpucache"

	// First run of milton
	cache, err := NewCpuCache(logPath)
	if err != nil {
		t.Fatalf("failed to create CpuCache: %v", err)
	}

	// Write some sample cpuCount data to the cache
	cache.Write(1) // index 1
	cache.Write(2) // index 2
	cache.Write(3) // index 3
	cache.Write(4) // index 4
	cache.Write(5) // index 5

	samples, lastIndex, _ := cache.GetAllSamples()
	if len(samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(samples))
	}
	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}

	cache.Close()

	// Second run of milton
	cache, _ = NewCpuCache(logPath)

	// There is a bug that if the cache was never truncated then the subsequent
	// runs of miltons will not get the first sample. Milton behaves like this
	// because of workaround for https://github.com/tidwall/wal/issues/20
	// It could be solved by recording the last index in a separate file. But
	// that migth be unnecessary added complexity and this behavior could be
	// acceptable.
	samples, lastIndex, _ = cache.GetAllSamples()
	if len(samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(samples))
	}
	if samples[0].Value != 2 {
		t.Fatalf("expected first sample to have value of 2, got %f", samples[0].Value)
	}
	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}

	cache.TruncateTo(lastIndex)
	cache.Close()

	// Third run of milton - after cache was truncated
	cache, _ = NewCpuCache(logPath)
	defer cache.Close()

	samples, lastIndex, _ = cache.GetAllSamples()
	if len(samples) != 0 {
		t.Fatalf("expected that after truncation the samples to send is 0, got %d", len(samples))
	}

	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}
}
