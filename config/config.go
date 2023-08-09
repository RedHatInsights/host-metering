package config

import "fmt"

type Config struct {
	WriteUrl        string
	WriteInterval   uint // in seconds
	HostCertPath    string
	HostCertKeyPath string
}

func NewConfig(writeUrl string, writeInterval uint, certPath string, keyPath string) *Config {
	return &Config{
		WriteUrl:        writeUrl,
		WriteInterval:   writeInterval,
		HostCertPath:    certPath,
		HostCertKeyPath: keyPath,
	}
}

func (c *Config) Print() {
	fmt.Println("Config:")
	fmt.Println("  WriteUrl: ", c.WriteUrl)
	fmt.Println("  WriteInterval: ", c.WriteInterval)
	fmt.Println("  HostCertPath: ", c.HostCertPath)
	fmt.Println("  HostCertKeyPath: ", c.HostCertKeyPath)
}
