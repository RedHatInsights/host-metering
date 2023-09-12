package notify

import (
	"testing"
)

// Test basic functionality of the MetricsLog - how it would be used by milton
func TestMetricsLogBasics(t *testing.T) {
	// Create a new MetricsLog instance
	dir := t.TempDir()
	logPath := dir + "/metrics"
	log, err := NewMetricsLog(logPath)
	if err != nil {
		t.Fatalf("failed to create MetricsLog: %v", err)
	}
	defer log.Close()

	// Write some sample cpuCount data to the log
	err = log.Write(4)
	if err != nil {
		t.Fatalf("failed to write sample data to MetricsLog: %v", err)
	}

	// Another sample
	err = log.Write(6)
	if err != nil {
		t.Fatalf("failed to write sample data to MetricsLog: %v", err)
	}

	// Get all samples from the log
	samples, lastIndex, err := log.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from MetricsLog: %v", err)
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

	// Truncate the log to the last item read
	err = log.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate MetricsLog: %v", err)
	}

	// Get all samples from the log again, to check that the truncation worked
	samples, lastIndex, err = log.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from MetricsLog: %v", err)
	}

	// Verify that the log is now empty
	if len(samples) != 0 {
		t.Fatalf("expected 0 samples, got %d", len(samples))
	}
	// Last index should still the same
	if lastIndex != 2 {
		t.Fatalf("expected last index of 2, got %d", lastIndex)
	}

	// Try again, but this time there is nothing to truncate
	err = log.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate MetricsLog when there is nothing to truncate: %v", err)
	}

	// Simulate next iteration: write sample, obtain it and truncate
	err = log.Write(8)
	if err != nil {
		t.Fatalf("failed to write sample data to MetricsLog: %v", err)
	}
	samples, lastIndex, err = log.GetAllSamples()
	if err != nil {
		t.Fatalf("failed to get samples from MetricsLog: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].Value != 8 {
		t.Fatalf("expected sample value of 8, got %f", samples[0].Value)
	}
	err = log.TruncateTo(lastIndex)
	if err != nil {
		t.Fatalf("failed to truncate MetricsLog: %v", err)
	}

}

// Test scenario where Prometheus server is not initially reachable
// (log is not truncated). And milton is restarted in the meantime.
func TestRestart(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/metrics"

	// First run of milton
	log, err := NewMetricsLog(logPath)
	if err != nil {
		t.Fatalf("failed to create MetricsLog: %v", err)
	}

	// Write some sample cpuCount data to the log
	log.Write(1) // index 1
	log.Write(2) // index 2
	log.Write(3) // index 3
	log.Write(4) // index 4
	log.Write(5) // index 5

	samples, lastIndex, _ := log.GetAllSamples()
	if len(samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(samples))
	}
	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}

	log.Close()

	// Second run of milton
	log, _ = NewMetricsLog(logPath)

	// There is a bug that if the log was never truncated then the subsequent
	// runs of miltons will not get the first sample. Milton behaves like this
	// because of workaround for https://github.com/tidwall/wal/issues/20
	// It could be solved by recording the last index in a separate file. But
	// that migth be unnecessary added complexity and this behavior could be
	// acceptable.
	samples, lastIndex, _ = log.GetAllSamples()
	if len(samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(samples))
	}
	if samples[0].Value != 2 {
		t.Fatalf("expected first sample to have value of 2, got %f", samples[0].Value)
	}
	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}

	log.TruncateTo(lastIndex)
	log.Close()

	// Third run of milton - after log was truncated
	log, _ = NewMetricsLog(logPath)
	defer log.Close()

	samples, lastIndex, _ = log.GetAllSamples()
	if len(samples) != 0 {
		t.Fatalf("expected that after truncation the samples to send is 0, got %d", len(samples))
	}

	if lastIndex != 5 {
		t.Fatalf("expected last index of 5, got %d", lastIndex)
	}
}
