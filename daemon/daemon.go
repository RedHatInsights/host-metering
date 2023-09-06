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
	config   *config.Config
	hostInfo *hostinfo.HostInfo
	cpuCache *notify.CpuCache
}

func NewDaemon(config *config.Config) *Daemon {
	return &Daemon{config: config}
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
	if d.config.CollectIntervalSec > 0 {
		collectTicker = time.NewTicker(time.Duration(d.config.CollectIntervalSec) * time.Second)
	} else {
		// Create dummy stopped ticker if collect interval is not configured
		collectTicker = time.NewTicker(time.Duration(1) * time.Hour)
		collectTicker.Stop()
	}

	writeTicker := time.NewTicker(time.Duration(d.config.WriteIntervalSec) * time.Second)

	if err := d.loadHostInfo(); err != nil {
		return err
	}
	if err := d.initCpuCache(); err != nil {
		return err
	}

	certWatcher, err := hostinfo.NewCertWatcher(d.config.HostCertPath)
	if err != nil {
		// CertWatch failure should not be fatal
		fmt.Println(err)
	}

	go func() {
		for {
			select {
			case <-collectTicker.C:
				d.collectMetrics()
			case <-writeTicker.C:
				if d.config.CollectIntervalSec == 0 {
					d.collectMetrics()
				}
				d.doPrometheusRequest()
			case <-reloadCh:
				logger.Infoln("Reloading HostInfo...")
				if err := d.loadHostInfo(); err != nil {
					logger.Errorln(err.Error())
					continue
				}
				logger.Infoln("HostInfo reloaded")
			case event, ok := <-certWatcher.Event:
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
	if err := d.loadHostInfo(); err != nil {
		return err
	}
	if err := d.initCpuCache(); err != nil {
		return err
	}
	logger.Infoln("Executing once...")
	d.collectMetrics()
	err := d.doPrometheusRequest()
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

func (d *Daemon) initCpuCache() error {
	logger.Debugln("Initializing CPU cache...")
	cache, err := notify.NewCpuCache(d.config.CpuCachePath)
	if err != nil {
		logger.Errorln(err.Error())
		return err
	}
	d.cpuCache = cache
	logger.Debugln("CPU cache initialized")
	return nil
}

func (d *Daemon) collectMetrics() {
	logger.Debugln("Collecting metrics...")

	err := d.hostInfo.RefreshCpuCount()
	if err != nil {
		logger.Warnf("Error refreshing CPU count: %s\n", err.Error())
		return
	}

	err = d.cpuCache.Write(d.hostInfo.CpuCount)
	if err != nil {
		logger.Warnf("Error writing CPU cache: %s\n", err.Error())
		return
	}
	logger.Debugln("Metrics collected")
}

func (d *Daemon) doPrometheusRequest() error {
	if d.hostInfo == nil {
		return fmt.Errorf("missing internal HostInfo")
	}
	logger.Debugln("Initiating Prometheus request...")
	samples, lastIndex, err := d.cpuCache.GetAllSamples()
	if err != nil {
		logger.Warnf("Error getting samples: %s\n", err.Error())
		return err
	}
	count := len(samples)
	if count == 0 {
		logger.Debugln("No samples to send")
		return nil
	}
	logger.Debugf("Sending %d sample(s)...\n", count)
	err = notify.PrometheusRemoteWrite(d.hostInfo, d.config, samples)
	if err != nil {
		logger.Warnf("Error calling PrometheusRemoteWrite: %s\n", err.Error())
		return err
	}
	err = d.cpuCache.TruncateTo(lastIndex)
	if err != nil {
		logger.Warnf("Error truncating WAL: %s\n", err.Error())
		return err
	}
	logger.Debugln("Prometheus remote write successful")
	return nil
}
