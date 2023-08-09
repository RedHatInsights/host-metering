package config

import "fmt"

type Config struct {
	WriteUrl        string
	Tick            uint // in seconds
	HostCertPath    string
	HostCertKeyPath string
}

func NewConfig(writeUrl string, tick uint, certPath string, keyPath string) *Config {
	return &Config{
		WriteUrl:        writeUrl,
		Tick:            tick,
		HostCertPath:    certPath,
		HostCertKeyPath: keyPath,
	}
}

func (c *Config) Print() {
	fmt.Println("Config:")
	fmt.Println("  WriteUrl: ", c.WriteUrl)
	fmt.Println("  Tick: ", c.Tick)
	fmt.Println("  HostCertPath: ", c.HostCertPath)
	fmt.Println("  HostCertKeyPath: ", c.HostCertKeyPath)
}
