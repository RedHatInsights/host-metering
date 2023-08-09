package config

import "fmt"

type Config struct {
	WriteUrl             string
	WriteInterval        uint // in seconds
	CollectInterval      uint // in seconds
	LabelRefreshInterval uint // in seconds
	HostCertPath         string
	HostCertKeyPath      string
	WriteRetryAttempts   uint
	WriteRetryInterval   uint // in seconds
}

func NewConfig(writeUrl string, writeInterval uint, certPath string, keyPath string) *Config {
	return &Config{
		WriteUrl:             writeUrl,
		WriteInterval:        writeInterval,
		HostCertPath:         certPath,
		HostCertKeyPath:      keyPath,
		CollectInterval:      0,
		LabelRefreshInterval: 86400,
		WriteRetryAttempts:   3,
		WriteRetryInterval:   1,
	}
}

func (c *Config) Print() {
	fmt.Println("Config:")
	fmt.Println("  WriteUrl: ", c.WriteUrl)
	fmt.Println("  WriteInterval: ", c.WriteInterval)
	fmt.Println("  HostCertPath: ", c.HostCertPath)
	fmt.Println("  HostCertKeyPath: ", c.HostCertKeyPath)
	fmt.Println("  CollectInterval: ", c.CollectInterval)
	fmt.Println("  LabelRefreshInterval: ", c.LabelRefreshInterval)
	fmt.Println("  WriteRetryAttempts: ", c.WriteRetryAttempts)
	fmt.Println("  WriteRetryInterval: ", c.WriteRetryInterval)
}
