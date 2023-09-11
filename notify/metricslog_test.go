package notify

import (
	"github.com/prometheus/prometheus/prompb"
	"testing"
)

// Test basic functionality of the MetricsLog - how it would be used by milton
func TestMetricsLogBasics(t *testing.T) {
	// Create a new MetricsLog instance
	log, err := NewMetricsLog(createMetricsPath(t))
	checkError(t, err, "failed to create MetricsLog")
	defer log.Close()

	// Write some sample cpuCount data to the log
	err = log.WriteSample(4)
	checkError(t, err, "failed to write sample data to MetricsLog")

	// Another sample
	err = log.WriteSample(6)
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
	err = log.WriteSample(8)
	checkError(t, err, "failed to write sample data to MetricsLog")

	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 8)
	checkIndex(t, checkpoint, 5)

	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")
}

// Test scenario where Prometheus server is not initially reachable
// (log is not truncated). And milton is restarted in the meantime.
func TestRestart(t *testing.T) {
	logPath := createMetricsPath(t)

	// First run of milton
	log, err := NewMetricsLog(logPath)
	checkError(t, err, "failed to create MetricsLog")

	// Write some sample cpuCount data to the log
	log.WriteSample(1) // index 1
	log.WriteSample(2) // index 2
	log.WriteSample(3) // index 3
	log.WriteSample(4) // index 4
	log.WriteSample(5) // index 5

	samples, checkpoint, err := log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3, 4, 5)
	checkIndex(t, checkpoint, 6)

	err = log.Close()
	checkError(t, err, "failed to close MetricsLog")

	// Second run of milton
	log, _ = NewMetricsLog(logPath)

	samples, checkpoint, err = log.GetSamples()
	checkError(t, err, "failed to get samples from MetricsLog")
	checkSamples(t, samples, 1, 2, 3, 4, 5)
	checkIndex(t, checkpoint, 6)

	err = log.RemoveSamples(checkpoint)
	checkError(t, err, "failed to truncate MetricsLog")

	err = log.Close()
	checkError(t, err, "failed to close MetricsLog")

	// Third run of milton - after log was truncated
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

func checkError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
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
