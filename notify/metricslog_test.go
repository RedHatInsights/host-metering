package notify

import (
	"os"
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/tidwall/wal"
)

// Test invalid operations with the MetricsLog.
func TestMetricsLogFailures(t *testing.T) {
	// Test an invalid log path.
	_, err := NewMetricsLog("")
	checkExpectedError(t, err, "metrics log path cannot be empty")

	// Test a corrupted log file.
	path := createMetricsPath(t)
	createCorruptedMetrics(t, path)

	_, err = NewMetricsLog(path)
	checkExpectedError(t, err, "log corrupt")

	// Create a valid log.
	path = createMetricsPath(t)
	log, _ := NewMetricsLog(path)

	// Test an invalid checkpoint.
	err = log.RemoveSamples(0)
	checkExpectedError(t, err, "out of range")

	// Close the log.
	_ = log.Close()

	// Test an invalid write operation.
	err = log.WriteSampleNow(2)
	checkExpectedError(t, err, "log closed")

	// Test an invalid read operation.
	_, _, err = log.GetSamples()
	checkExpectedError(t, err, "log closed")

	// Test an invalid delete operation.
	err = log.RemoveSamples(2)
	checkExpectedError(t, err, "log closed")

	// Test an invalid close operation.
	err = log.Close()
	checkExpectedError(t, err, "log closed")
}

// Test checkpoint support of the MetricsLog.
func TestMetricsLogCheckpoints(t *testing.T) {
	// Create a new MetricsLog instance
	log, err := NewMetricsLog(createMetricsPath(t))
	checkError(t, err, "failed to create MetricsLog")
	defer log.Close()

	// Get samples from an empty log.
	samples, checkpoint, err := log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples)
	checkIndex(t, checkpoint, 1)

	// Get samples from an empty log again.
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples)
	checkIndex(t, checkpoint, 1)

	// Truncate an empty log.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Truncate an empty log again.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Write something into the log.
	_ = log.WriteSampleNow(1)
	_ = log.WriteSampleNow(2)
	_ = log.WriteSampleNow(3)

	// Get samples from the log.
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3)
	checkIndex(t, checkpoint, 5)

	// Get samples from the log again.
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3)
	checkIndex(t, checkpoint, 5)

	// Truncate the log.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Truncate the empty log.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Write something new into the log.
	_ = log.WriteSampleNow(4)
	_ = log.WriteSampleNow(5)

	// Get samples from the log.
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 4, 5)
	checkIndex(t, checkpoint, 8)

	// Get samples from the log again.
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 4, 5)
	checkIndex(t, checkpoint, 8)

	// Truncate the log.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Truncate the empty log.
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")
}

// Test basic functionality of the MetricsLog - how it would be used by host-metering
func TestMetricsLogBasics(t *testing.T) {
	// Create a new MetricsLog instance
	log, err := NewMetricsLog(createMetricsPath(t))
	checkError(t, err, "failed to create MetricsLog")
	defer log.Close()

	// Write some sample cpuCount data to the log
	err = log.WriteSampleNow(4)
	checkError(t, err, "failed to write sample data to MetricsLog")

	// Another sample
	err = log.WriteSampleNow(6)
	checkError(t, err, "failed to write sample data to MetricsLog")

	// Get all samples from the log
	samples, checkpoint, err := log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 4, 6)
	checkIndex(t, checkpoint, 3)

	// Truncate the log to the last item read
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Get all samples from the log again, to check that the truncation worked
	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples)
	checkIndex(t, checkpoint, 3)

	// Try again, but this time there is nothing to truncate
	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	// Simulate next iteration: write sample, obtain it and truncate
	err = log.WriteSampleNow(8)
	checkError(t, err, "failed to write sample data to MetricsLog")

	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 8)
	checkIndex(t, checkpoint, 5)

	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")
}

// Test scenario where Prometheus server is not initially reachable
// (log is not truncated). And host-metering is restarted in the meantime.
func TestRestart(t *testing.T) {
	logPath := createMetricsPath(t)

	// First run of host-metering
	log, err := NewMetricsLog(logPath)
	checkError(t, err, "failed to create MetricsLog")

	// Write some sample cpuCount data to the log
	log.WriteSampleNow(1) // index 1
	log.WriteSampleNow(2) // index 2
	log.WriteSampleNow(3) // index 3
	log.WriteSampleNow(4) // index 4
	log.WriteSampleNow(5) // index 5

	samples, checkpoint, err := log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3, 4, 5)
	checkIndex(t, checkpoint, 6)

	err = log.Close()
	checkError(t, err, "failed to close MetricsLog")

	// Second run of host-metering
	log, _ = NewMetricsLog(logPath)

	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3, 4, 5)
	checkIndex(t, checkpoint, 6)

	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	err = log.Close()
	checkError(t, err, "failed to close MetricsLog")

	// Third run of host-metering - after log was truncated
	log, err = NewMetricsLog(logPath)
	checkError(t, err, "failed to create MetricsLog")
	defer log.Close()

	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples)
	checkIndex(t, checkpoint, 6)
}

func createMetricsPath(t *testing.T) string {
	dir := t.TempDir()
	return dir + "/metrics"
}

func createCorruptedMetrics(t *testing.T, path string) {
	err := os.MkdirAll(path, wal.DefaultOptions.DirPerms)
	if err != nil {
		t.Fatalf("failed to create a directory at %s: %v", path, err)
	}

	err = os.WriteFile(path+"/00000000000000000001", []byte("\n"), wal.DefaultOptions.FilePerms)
	if err != nil {
		t.Fatalf("failed to create a corrupted file at %s: %v", path, err)
	}
}

func checkError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

func checkExpectedError(t *testing.T, err error, message string) {
	if err == nil {
		t.Fatalf("expected error with message: %s", message)
	}
	if err.Error() != message {
		t.Fatalf("unexpected error message: '%s' != '%s'", err.Error(), message)
	}
}

func checkIndex(t *testing.T, index uint64, expected uint64) {
	if index != expected {
		t.Fatalf("unexpected index: %d != %d", index, expected)
	}
}

func checkSamples(t *testing.T, samples []prompb.Sample, expected ...float64) {
	// Check the expected number of samples.
	if len(samples) != len(expected) {
		t.Fatalf("unexpected number of samples: %d != %d", len(samples), len(expected))
	}

	// Check the expected values of samples.
	for i := 0; i < len(samples); i++ {
		if samples[i].Value != expected[i] {
			t.Fatalf("unexpected value of the sample %d: %f != %f", i, samples[i].Value, expected[i])
		}
	}

	// Check timestamps of the samples.
	for i := 1; i < len(samples); i++ {
		if samples[i].Timestamp < samples[i-1].Timestamp {
			t.Fatalf("unexpected timestamp of the sample %d: %d < %d",
				i, samples[i].Timestamp, samples[i-1].Timestamp)
		}
	}
}
