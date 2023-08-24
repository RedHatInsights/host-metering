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

	ticker := time.NewTicker(time.Duration(d.config.WriteInterval) * time.Second)
	if err := d.loadHostInfo(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				d.doPrometheusRequest()
			case <-reloadCh:
				fmt.Println("Reloading HostInfo...")
				if err := d.loadHostInfo(); err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println("HostInfo reloaded")
			case <-stopCh:
				ticker.Stop()
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
	fmt.Println("Executing once...")
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

func (d *Daemon) doPrometheusRequest() error {
	if d.hostInfo == nil {
		return fmt.Errorf("missing internal HostInfo")
	}
	fmt.Println("Sending Prometheus request...")
	err := notify.PrometheusRemoteWrite(d.hostInfo, d.config)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Prometheus remote write successful")
	return nil
}
