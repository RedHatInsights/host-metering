package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
)

func TestNotify(t *testing.T) {
	mlPath := createMetricsPath(t)
	daemon, err := NewDaemon(&config.Config{
		MetricsMaxAge:  10 * time.Second,
		MetricsWALPath: mlPath,
	})
	checkError(t, err, "failed to create daemon")
	metricsLog := daemon.metricsLog
	notifier := &mockNotifier{}
	daemon.notifier = notifier
	daemon.hostInfo = &hostinfo.HostInfo{}

	// Test that notifier is called when there are some samples
	metricsLog.WriteSampleNow(1)
	notifier.ExpectSuccess()
	err = daemon.notify()
	checkError(t, err, "failed to notify")
	notifier.CheckWasCalled(t)

	// Test that notifier was called with the sample and hostinfo
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(notifier.calledWith.samples))
	}
	if notifier.calledWith.hostinfo != daemon.hostInfo {
		t.Fatalf("expected hostinfo to be passed to notifier")
	}

	// Test that log is trunctated after notifying
	samples, _, _ := metricsLog.GetSamples()
	if len(samples) != 0 {
		t.Fatalf("expected log to be truncated")
	}

	// Test that notifier is not called when there are no samples
	notifier.ResetCalledWith()
	err = daemon.notify()
	checkError(t, err, "failed to notify")
	notifier.CheckWasNotCalled(t)

	// Test that expired samples are pruned even on error
	expiredTs := time.Now().UnixMilli() - 11000
	metricsLog.WriteSample(1, expiredTs)
	metricsLog.WriteSampleNow(2)
	notifier.ExpectError(errors.New("mocked error"))
	err = daemon.notify()
	checkExpectedError(t, err, "mocked error")
	samples, _, _ = metricsLog.GetSamples()
	if len(samples) != 1 {
		t.Fatalf("expected expired sample to be pruned")
	}
	if samples[0].Value != 2 {
		t.Fatalf("expected non-expired sample to be kept")
	}

	// Test that notifier was called only with non-expired samples
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(notifier.calledWith.samples))
	}
	if notifier.calledWith.samples[0].Value != 2 {
		t.Fatalf("expected non-expired sample to be passed to notifier")
	}

}

// Helper functions

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

// Mock/use metrics log

func createMetricsPath(t *testing.T) string {
	dir := t.TempDir()
	return dir + "/metrics"
}

// Mock Notifier

type notifyArgs struct {
	samples  []prompb.Sample
	hostinfo *hostinfo.HostInfo
}

type mockNotifier struct {
	calledWith *notifyArgs
	result     func(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error
}

func (n *mockNotifier) Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error {
	n.calledWith = &notifyArgs{samples, hostinfo}
	return n.result(samples, hostinfo)
}

func (n *mockNotifier) ResetCalledWith() {
	n.calledWith = nil
}

func (n *mockNotifier) CheckWasCalled(t *testing.T) {
	if n.calledWith == nil {
		t.Fatalf("expected notifier to be called")
	}
}

func (n *mockNotifier) CheckWasNotCalled(t *testing.T) {
	if n.calledWith != nil {
		t.Fatalf("expected notifier to not be called")
	}
}

func (n *mockNotifier) ExpectError(err error) {
	n.result = func(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error {
		return err
	}
}

func (n *mockNotifier) ExpectSuccess() {
	n.result = func(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error {
		return nil
	}
}
