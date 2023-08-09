package config

import "fmt"

const (
	DefaultConfigPath           = "/etc/milton.conf"
	DefaultWriteUrl             = "http://localhost:9090/api/v1/write"
	DefaultWriteInterval        = 600
	DefaultCertPath             = "/etc/pki/consumer/cert.pem"
	DefaultKeyPath              = "/etc/pki/consumer/key.pem"
	DefaultCollectInterval      = 0
	DefaultLabelRefreshInterval = 86400
	DefaultWriteRetryAttempts   = 3
	DefaultWriteRetryInterval   = 1
)

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
		CollectInterval:      DefaultCollectInterval,
		LabelRefreshInterval: DefaultLabelRefreshInterval,
		WriteRetryAttempts:   DefaultWriteRetryAttempts,
		WriteRetryInterval:   DefaultWriteRetryInterval,
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
