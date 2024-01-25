package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/host-metering/config"
	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/RedHatInsights/host-metering/logger"
	"github.com/RedHatInsights/host-metering/notify"
)

type Daemon struct {
	config           *config.Config
	hostInfo         *hostinfo.HostInfo
	hostInfoProvider hostinfo.HostInfoProvider
	metricsLog       *notify.MetricsLog
	certWatcher      hostinfo.CertWatcher
	notifier         notify.Notifier
	notifyPolicy     notify.NotifyPolicy
	stopCh           chan os.Signal
	started          bool
}

func NewDaemon(config *config.Config) (*Daemon, error) {
	var err error
	d := &Daemon{
		config:           config,
		notifier:         notify.NewPrometheusNotifier(config),
		hostInfoProvider: &hostinfo.SubManInfoProvider{},
		notifyPolicy:     &notify.GeneralNotifyPolicy{},
	}
	d.certWatcher, err = hostinfo.NewINotifyCertWatcher(d.config.HostCertPath)
	if err != nil {
		// CertWatch failure should not be fatal
		logger.Errorf("Cert Watcher initialization failed: %v\n", err.Error())
	}
	if err := d.initMetricsLog(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Daemon) Run() error {
	d.started = false
	logger.Infoln("Starting server...")

	// Wait for SIGINT or SIGTERM to stop server
	d.stopCh = make(chan os.Signal, 1)
	signal.Notify(d.stopCh, syscall.SIGINT, syscall.SIGTERM)
	shutdownCh := make(chan int)

	// Wait for SIGHUP to reload host info
	reloadCh := make(chan os.Signal, 1)
	signal.Notify(reloadCh, syscall.SIGHUP)

	err := d.initialNotify()
	if err != nil {
		logger.Errorln(err.Error())
	}

	var certWatchEvent chan hostinfo.CertEvent
	if d.certWatcher != nil {
		certWatchEvent = d.certWatcher.Event()
	}

	var collectTicker *time.Ticker
	if d.config.CollectInterval > 0 {
		collectTicker = time.NewTicker(d.config.CollectInterval)
		defer collectTicker.Stop()
	} else {
		// Create dummy stopped ticker if collect interval is not configured
		collectTicker = time.NewTicker(time.Duration(1) * time.Hour)
		collectTicker.Stop()
	}

	writeTicker := time.NewTicker(d.config.WriteInterval)
	defer writeTicker.Stop()

	var labelTicker *time.Ticker
	if d.config.LabelRefreshInterval > 0 {
		labelTicker = time.NewTicker(d.config.LabelRefreshInterval)
		defer labelTicker.Stop()
	} else {
		// Create dummy stopped ticker if label interval is not configured
		labelTicker = time.NewTicker(time.Duration(1) * time.Hour)
		labelTicker.Stop()
	}

	go func() {
		for {
			select {
			case <-collectTicker.C:
				d.collectMetrics()
			case <-writeTicker.C:
				if d.config.CollectInterval == 0 {
					d.collectMetrics()
				}
				d.notify()
			case <-labelTicker.C:
				logger.Infoln("Refresh labels...")
				if err := d.loadHostInfo(); err != nil {
					logger.Errorln(err.Error())
					continue
				}
				logger.Infoln("Labels refreshed")
			case <-reloadCh:
				logger.Infoln("Reloading HostInfo...")
				if err := d.loadHostInfo(); err != nil {
					logger.Errorln(err.Error())
					continue
				}
				logger.Infoln("HostInfo reloaded")
			case event, ok := <-certWatchEvent:
				if !ok {
					continue
				}
				switch event {
				case hostinfo.WriteEvent:
					logger.Infoln("Host cert updated")
				case hostinfo.RemoveEvent:
					logger.Infoln("Host cert removed")
				}
				if err := d.loadHostInfo(); err != nil {
					logger.Errorf("Host info load error: %s\n", err.Error())
				}
			case <-d.stopCh:
				d.stopCh = nil
				shutdownCh <- 1
				return
			}
		}
	}()
	logger.Infoln("Server fully started")
	d.started = true
	<-shutdownCh
	d.started = false
	logger.Infoln("Server stopped")
	return nil
}

func (d *Daemon) RunOnce() error {
	logger.Infoln("Executing once...")
	return d.initialNotify()
}

func (d *Daemon) Stop() {
	logger.Infoln("Initiating stop...")
	if d.stopCh != nil || !d.started {
		d.stopCh <- syscall.SIGTERM
	} else {
		logger.Infoln("Server is not running")
	}
}

// Server is fully started (initial notification done, timers active)
func (d *Daemon) IsStarted() bool {
	return d.started
}

// initialNotify collects data and does an initial notification
func (d *Daemon) initialNotify() error {
	if err := d.loadHostInfo(); err != nil {
		return err
	}
	d.collectMetrics()
	err := d.notify()
	return err
}

func (d *Daemon) loadHostInfo() error {
	logger.Debugln("Load HostInfo...")
	hostInfo, err := d.hostInfoProvider.Load()
	if err != nil {
		return err
	}
	logger.Infoln("HostInfo loaded")
	logger.Infoln(hostInfo.String())
	d.hostInfo = hostInfo
	d.notifier.HostChanged()
	return nil
}

func (d *Daemon) initMetricsLog() error {
	logger.Debugln("Initializing metrics log...")
	log, err := notify.NewMetricsLog(d.config.MetricsWALPath)
	if err != nil {
		logger.Errorln(err.Error())
		return err
	}
	d.metricsLog = log
	logger.Debugln("Metrics log initialized")
	return nil
}

func (d *Daemon) collectMetrics() {
	logger.Debugln("Collecting metrics...")

	err := d.hostInfoProvider.RefreshCpuCount(d.hostInfo)
	if err != nil {
		logger.Warnf("Error refreshing CPU count: %s\n", err.Error())
		return
	}

	err = d.metricsLog.WriteSampleNow(d.hostInfo.CpuCount)
	if err != nil {
		logger.Warnf("Error writing metrics log: %s\n", err.Error())
		return
	}
	logger.Debugln("Metrics collected")
}

func (d *Daemon) notify() error {
	if d.hostInfo == nil {
		return fmt.Errorf("missing internal HostInfo")
	}
	logger.Debugln("Initiating notification request...")
	samples, checkpoint, err := d.metricsLog.GetSamples()
	if err != nil {
		logger.Warnf("Error getting samples: %s\n", err.Error())
		return err
	}
	origCount := len(samples)
	if d.config.MetricsMaxAge > 0 {
		samples = notify.FilterSamplesByAge(samples, d.config.MetricsMaxAge)
	}
	err = d.notifyPolicy.ShouldNotify(samples, d.hostInfo)
	if err != nil {
		logger.Warnf("Cannot notify: %s\n", err.Error())
		return nil
	}

	count := len(samples)
	logger.Debugf("Sending %d sample(s)...\n", count)
	err = d.notifier.Notify(samples, d.hostInfo)
	var notifyError *notify.NotifyError
	var truncateError error
	if err == nil {
		// clear all samples on success as they were accepted by the server
		logger.Infof("Notification successful - sent %d sample(s)\n", count)
		truncateError = d.metricsLog.RemoveSamples(checkpoint)
	} else if errors.As(err, &notifyError) && !notifyError.Recoverable() {
		// clear all samples on non-recoverable error
		logger.Warnf("Notification [%d sample(s)]): %s\n", count, notifyError.Error())
		truncateError = d.metricsLog.RemoveSamples(checkpoint)
	} else {
		// don't clear or clear only old so that WAL does not grow indefinitely on retries
		// on recoverable or unknowns errors
		logger.Warnf("Notification [%d sample(s)]: %s\n", count, err.Error())
		if origCount != count {
			logger.Infof("Dropping %d expired sample(s)\n", origCount-count)
			truncateError = d.metricsLog.RemoveOldestSamples(origCount - count)
		}
	}

	if truncateError != nil {
		logger.Warnf("Error truncating WAL: %s\n", truncateError.Error())
		return err
	}

	return err
}
