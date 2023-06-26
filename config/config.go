package config

import "fmt"

type Config struct {
	PrometheusUrl   string
	Tick            uint // in seconds
	HostCertPath    string
	HostCertKeyPath string
}

func NewConfig(prometheusUrl string, tick uint, certPath string, keyPath string) *Config {
	return &Config{
		PrometheusUrl:   prometheusUrl,
		Tick:            tick,
		HostCertPath:    certPath,
		HostCertKeyPath: keyPath,
	}
}

func (c *Config) Print() {
	fmt.Println("Config:")
	fmt.Println("  PrometheusUrl: ", c.PrometheusUrl)
	fmt.Println("  Tick: ", c.Tick)
	fmt.Println("  HostCertPath: ", c.HostCertPath)
	fmt.Println("  HostCertKeyPath: ", c.HostCertKeyPath)
}
