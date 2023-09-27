package daemon

import (
	"errors"
	"testing"
	"time"

	"github.com/RedHatInsights/host-metering/config"
	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/RedHatInsights/host-metering/notify"
	"github.com/prometheus/prometheus/prompb"
)

func TestRunOnce(t *testing.T) {
	daemon, notifier, _, _ := createDaemon(t)
	notifier.ExpectSuccess()

	// Test that daemon is running after starting
	err := daemon.RunOnce()
	checkError(t, err, "failed to run once")
	notifier.CheckWasCalled(t)
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(notifier.calledWith.samples))
	}
}

func TestRunAndStopping(t *testing.T) {
	daemon, notifier, _, _ := createDaemon(t)
	notifier.ExpectSuccess()

	// Test that daemon is not running before starting
	waitForStopped(t, daemon)

	// Test that daemon is running after starting
	go daemon.Run()
	waitForStarted(t, daemon)

	// Test that it can be started and stopped multiple times
	daemon.Stop()
	waitForStopped(t, daemon)

	go daemon.Run()
	waitForStarted(t, daemon)

	daemon.Stop()
	waitForStopped(t, daemon)
}

func TestRunWithCollect(t *testing.T) {
	daemon, notifier, metricsLog, _ := createDaemon(t)
	notifier.ExpectSuccess()
	daemon.config.CollectInterval = 20 * time.Millisecond
	daemon.config.WriteInterval = 30 * time.Millisecond

	// Test that deamon does initial notification on start before
	go daemon.Run()
	checkRunning(t, daemon)
	// Check initial notification (before fully started)
	notifier.WaitForCall(t, 10*time.Millisecond)
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected initial notification with one sample")
	}
	notifier.ResetCalledWith()
	waitForEmptyMetricsLog(t, metricsLog, 10*time.Millisecond)
	waitForStarted(t, daemon)

	// Collect on collect interval - verify it was collected but not sent
	time.Sleep(daemon.config.CollectInterval)
	notifier.CheckWasNotCalled(t)
	waitForValuesInMetricsLog(t, metricsLog, 1, 10*time.Millisecond)

	// Test that collect doesn't happen in notify when collect interval is set
	notifier.ExpectSuccess()
	notifier.ResetCalledWith()
	notifier.WaitForCall(t, daemon.config.WriteInterval-daemon.config.CollectInterval+1*time.Millisecond)
	waitForEmptyMetricsLog(t, metricsLog, 10*time.Millisecond)
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected that notify won't collect and will send 1 previous sample")
	}

	// Cleanup
	daemon.Stop()
	waitForStopped(t, daemon)
}

// Test that collect is done together with notify when collect interval is not set
func TestRunWithoutCollect(t *testing.T) {
	daemon, notifier, metricsLog, _ := createDaemon(t)
	daemon.config.CollectInterval = 0
	daemon.config.WriteInterval = 20 * time.Millisecond

	// Run initialization
	notifier.ResetCalledWith()
	notifier.ExpectSuccess()
	go daemon.Run()
	checkRunning(t, daemon)
	notifier.ResetCalledWith()
	waitForStarted(t, daemon)

	// Wait for and test the event
	notifier.WaitForCall(t, daemon.config.WriteInterval+10*time.Millisecond)
	waitForEmptyMetricsLog(t, metricsLog, 10*time.Millisecond)
	if len(notifier.calledWith.samples) != 1 {
		t.Fatalf("expected that notify will collect and send 1 sample")
	}

	// Cleanup
	daemon.Stop()
	waitForStopped(t, daemon)
}

// Test that HostInfo is reloaded on certificate change
func TestReloadOnCertChange(t *testing.T) {
	daemon, _, _, hostInfoProvider := createDaemon(t)
	certWatcher := daemon.certWatcher.(*mockCertWatcher)

	// Init
	go daemon.Run()
	waitForStarted(t, daemon)
	hostInfoProvider.ResetCalled()
	hostInfoProvider.WaitForCalled(t, 0)

	// Test that hostinfo is not reloaded on cert write
	certWatcher.ReportWriteEvent()
	hostInfoProvider.WaitForCalled(t, 1)

	// Test that hostinfo is reloaded on cert removal
	certWatcher.ReportRemoveEvent()
	hostInfoProvider.WaitForCalled(t, 2)

	// Test that it works on multiple events
	certWatcher.ReportWriteEvent()
	hostInfoProvider.WaitForCalled(t, 3)
	certWatcher.ReportWriteEvent()
	hostInfoProvider.WaitForCalled(t, 4)
	certWatcher.ReportRemoveEvent()
	hostInfoProvider.WaitForCalled(t, 5)
	certWatcher.ReportRemoveEvent()
	hostInfoProvider.WaitForCalled(t, 6)

	// Cleanup
	daemon.Stop()
	waitForStopped(t, daemon)
}

func TestNotify(t *testing.T) {
	daemon, notifier, metricsLog, hiProvider := createDaemon(t)
	daemon.config.MetricsMaxAge = 10 * time.Second
	daemon.hostInfo, _ = hiProvider.Load()

	// Test that notifier is called when there are some samples
	metricsLog.WriteSampleNow(1)
	notifier.ExpectSuccess()
	err := daemon.notify()
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
	checkEmptyMetricsLog(t, metricsLog)

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
	samples, _, _ := metricsLog.GetSamples()
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

// Wait and check if deamon run was initiated
func checkRunning(t *testing.T, daemon *Daemon) {
	timeout := time.NewTimer(10 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatalf("expected daemon to be running")
		default:
			if daemon.stopCh != nil {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// Wait and check if deamon is not started
func waitForStopped(t *testing.T, daemon *Daemon) {
	t.Helper()
	timeout := time.NewTimer(10 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatalf("expected daemon to be stoppped")
		default:
			if !daemon.IsStarted() {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// Wait and check if deamon is started
func waitForStarted(t *testing.T, daemon *Daemon) {
	t.Helper()
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatalf("expected daemon to be fully started")
		default:
			if daemon.IsStarted() {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func waitForEmptyMetricsLog(t *testing.T, metricsLog *notify.MetricsLog, timeout time.Duration) {
	t.Helper()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	for {
		select {
		case <-timeoutTimer.C:
			t.Fatalf("expected metrics log to be empty")
		default:
			samples, _, _ := metricsLog.GetSamples()
			if len(samples) == 0 {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func waitForValuesInMetricsLog(t *testing.T, metricsLog *notify.MetricsLog, count int, timeout time.Duration) {
	t.Helper()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	for {
		select {
		case <-timeoutTimer.C:
			t.Fatalf("expected metrics log to have %d values", count)
		default:
			samples, _, _ := metricsLog.GetSamples()
			if len(samples) == count {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func checkEmptyMetricsLog(t *testing.T, metricsLog *notify.MetricsLog) {
	t.Helper()
	samples, _, _ := metricsLog.GetSamples()
	if len(samples) != 0 {
		t.Fatalf("expected no values in metrics log")
	}
}

func checkError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

func checkExpectedError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with message: %s", message)
	}
	if err.Error() != message {
		t.Fatalf("unexpected error message: '%s' != '%s'", err.Error(), message)
	}
}

// Helper data init functions

func createDaemon(t *testing.T) (*Daemon, *mockNotifier, *notify.MetricsLog, *mockHostInfoProvider) {
	mlPath := createMetricsPath(t)
	config := config.NewConfig()
	config.MetricsWALPath = mlPath
	daemon, err := NewDaemon(config)
	notifier := &mockNotifier{}
	notifier.ExpectSuccess()
	daemon.notifier = notifier
	daemon.certWatcher = &mockCertWatcher{make(chan hostinfo.CertEvent)}
	hiProvider := newMockHostInfoProvider(&hostinfo.HostInfo{
		CpuCount:    2,
		HostId:      "testhost-id",
		SocketCount: "1",
		Product:     "testproduct",
		Support:     "testsupport",
		Usage:       "testusage",
		Billing: hostinfo.BillingInfo{
			Model:                 "testmodel",
			Marketplace:           "testmarketplace",
			MarketplaceAccount:    "testmarketplaceaccount",
			MarketplaceInstanceId: "testmarketplaceinstanceid",
		},
	})
	daemon.hostInfoProvider = hiProvider
	checkError(t, err, "failed to create daemon")
	return daemon, notifier, daemon.metricsLog, hiProvider
}

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
	t.Helper()
	if n.calledWith == nil {
		t.Fatalf("expected notifier to be called")
	}
}

func (n *mockNotifier) CheckWasNotCalled(t *testing.T) {
	t.Helper()
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

func (n *mockNotifier) WaitForCall(t *testing.T, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	for {
		if n.calledWith != nil {
			return
		}
		if time.Since(start) > timeout {
			t.Fatalf("expected notifier to be called")
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// Mock HostInfo provider

type mockHostInfoProvider struct {
	called uint
	hi     *hostinfo.HostInfo
}

func newMockHostInfoProvider(hi *hostinfo.HostInfo) *mockHostInfoProvider {
	return &mockHostInfoProvider{0, hi}
}

func (m *mockHostInfoProvider) Load() (*hostinfo.HostInfo, error) {
	m.called++
	return m.hi, nil
}

func (m *mockHostInfoProvider) RefreshCpuCount(hi *hostinfo.HostInfo) error {
	return nil
}

func (m *mockHostInfoProvider) ProviderCalled() uint {
	return m.called
}

func (m *mockHostInfoProvider) ResetCalled() {
	m.called = 0
}

func (m *mockHostInfoProvider) WaitForCalled(t *testing.T, n uint) {
	t.Helper()
	start := time.Now()
	for {
		if m.called == n {
			return
		}
		if time.Since(start) > 10*time.Millisecond {
			t.Fatalf("expected hostinfo provider to be called %d times, got %d", n, m.called)
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// Mock CertWatcher

type mockCertWatcher struct {
	event chan hostinfo.CertEvent
}

func (m *mockCertWatcher) Event() chan hostinfo.CertEvent {
	return m.event
}

func (m *mockCertWatcher) Close() {
}

func (m *mockCertWatcher) ReportWriteEvent() {
	m.event <- hostinfo.WriteEvent
}

func (m *mockCertWatcher) ReportRemoveEvent() {
	m.event <- hostinfo.RemoveEvent
}
