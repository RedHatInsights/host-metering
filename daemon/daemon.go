package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
	"redhat.com/milton/logger"
	"redhat.com/milton/notify"
)

type Daemon struct {
	config      *config.Config
	hostInfo    *hostinfo.HostInfo
	metricsLog  *notify.MetricsLog
	certWatcher *hostinfo.CertWatcher
	notifier    notify.Notifier
}

func NewDaemon(config *config.Config) (*Daemon, error) {
	var err error
	d := &Daemon{
		config:   config,
		notifier: notify.NewPrometheusNotifier(config),
	}
	d.certWatcher, err = hostinfo.NewCertWatcher(d.config.HostCertPath)
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
	logger.Infoln("Starting server...")

	// Wait for SIGINT or SIGTERM to stop server
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for SIGHUP to reload host ifo
	reloadCh := make(chan os.Signal, 1)
	signal.Notify(reloadCh, syscall.SIGHUP)

	var collectTicker *time.Ticker
	if d.config.CollectInterval > 0 {
		collectTicker = time.NewTicker(d.config.CollectInterval)
	} else {
		// Create dummy stopped ticker if collect interval is not configured
		collectTicker = time.NewTicker(time.Duration(1) * time.Hour)
		collectTicker.Stop()
	}

	writeTicker := time.NewTicker(d.config.WriteInterval)

	err := d.initialNotify()
	if err != nil {
		logger.Errorln(err.Error())
	}

	var certWatchEvent chan hostinfo.CertEvent
	if d.certWatcher != nil {
		certWatchEvent = d.certWatcher.Event
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
			case <-stopCh:
				collectTicker.Stop()
				writeTicker.Stop()
				return
			}
		}
	}()

	<-stopCh
	logger.Infoln("Stopping server...")
	return nil
}

func (d *Daemon) RunOnce() error {
	logger.Infoln("Executing once...")
	return d.initialNotify()
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
	hostInfo, err := hostinfo.LoadHostInfo()
	if err != nil {
		return err
	}
	logger.Infoln("HostInfo loaded")
	logger.Infoln(hostInfo.String())
	d.hostInfo = hostInfo
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

	err := d.hostInfo.RefreshCpuCount()
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
	count := len(samples)
	if count == 0 {
		logger.Debugln("No samples to send")
		return nil
	}
	logger.Debugf("Sending %d sample(s)...\n", count)
	err = d.notifier.Notify(samples, d.hostInfo)
	if err != nil {
		logger.Warnf("Error calling PrometheusRemoteWrite: %s\n", err.Error())
		// clear old samples even on error so that WAL does not grow indefinitely
		if origCount != count {
			err2 := d.metricsLog.RemoveSamples(checkpoint - uint64(count))
			if err2 != nil {
				logger.Warnf("Error truncating WAL: %s\n", err2.Error())
			}
		}
		return err
	}
	err = d.metricsLog.RemoveSamples(checkpoint)
	if err != nil {
		logger.Warnf("Error truncating WAL: %s\n", err.Error())
		return err
	}
	logger.Debugln("Notification successful")
	return nil
}
