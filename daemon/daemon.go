package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
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
	fmt.Println("Starting server...")

	// Wait for SIGINT or SIGTERM to stop server
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for SIGHUP to reload host ifo
	reloadCh := make(chan os.Signal, 1)
	signal.Notify(reloadCh, syscall.SIGHUP)

	var collectTicker *time.Ticker
	if d.config.CollectInterval > 0 {
		collectTicker = time.NewTicker(time.Duration(d.config.CollectInterval) * time.Second)
	} else {
		// Create dummy stopped ticker if collect interval is not configured
		collectTicker = time.NewTicker(time.Duration(1) * time.Hour)
		collectTicker.Stop()
	}

	writeTicker := time.NewTicker(time.Duration(d.config.WriteInterval) * time.Second)

	if err := d.loadHostInfo(); err != nil {
		return err
	}
	if err := d.initCpuCache(); err != nil {
		return err
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
				d.doPrometheusRequest()
			case <-reloadCh:
				fmt.Println("Reloading HostInfo...")
				if err := d.loadHostInfo(); err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println("HostInfo reloaded")
			case <-stopCh:
				collectTicker.Stop()
				writeTicker.Stop()
				return
			}
		}
	}()

	<-stopCh
	fmt.Println("Stopping server...")
	return nil
}

func (d *Daemon) RunOnce() error {
	if err := d.loadHostInfo(); err != nil {
		return err
	}
	if err := d.initCpuCache(); err != nil {
		return err
	}
	fmt.Println("Executing once...")
	d.collectMetrics()
	err := d.doPrometheusRequest()
	return err
}

func (d *Daemon) loadHostInfo() error {
	fmt.Println("Load HostInfo...")
	hostInfo, err := hostinfo.LoadHostInfo(d.config)
	if err != nil {
		return err
	}
	fmt.Println("HostInfo reloaded")
	d.hostInfo = hostInfo
	return nil
}

func (d *Daemon) initCpuCache() error {
	fmt.Println("Initializing CPU cache...")
	cache, err := notify.NewCpuCache(d.config.CpuCachePath)
	if err != nil {
		fmt.Println(err)
		return err
	}
	d.cpuCache = cache
	fmt.Println("CPU cache initialized")
	return nil
}

func (d *Daemon) collectMetrics() {
	fmt.Println("Collecting metrics...")

	d.hostInfo.RefreshCpuCount()
	err := d.cpuCache.Write(d.hostInfo.CpuCount)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Metrics collected")
}

func (d *Daemon) doPrometheusRequest() error {
	if d.hostInfo == nil {
		return fmt.Errorf("missing internal HostInfo")
	}
	fmt.Println("Initiating Prometheus request...")
	samples, lastIndex, err := d.cpuCache.GetAllSamples()
	if err != nil {
		fmt.Println(err)
		return err
	}
	count := len(samples)
	if count == 0 {
		fmt.Println("No samples to send")
		return nil
	}
	fmt.Println("Sending ", count, " sample(s)...")
	err = notify.PrometheusRemoteWrite(d.hostInfo, d.config, samples)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = d.cpuCache.TruncateTo(lastIndex)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Prometheus remote write successful")
	return nil
}
