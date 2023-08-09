package config

import (
	"fmt"
	"os"
	"strconv"
)

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

func NewConfig() *Config {
	return &Config{
		WriteUrl:             DefaultWriteUrl,
		WriteInterval:        DefaultWriteInterval,
		HostCertPath:         DefaultCertPath,
		HostCertKeyPath:      DefaultKeyPath,
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

func (c *Config) UpdateFromCliOptions(writeUrl string, writeInterval uint, certPath string, keyPath string) {
	if writeUrl != "" {
		c.WriteUrl = writeUrl
	}
	if writeInterval != 0 {
		c.WriteInterval = writeInterval
	}
	if certPath != "" {
		c.HostCertPath = certPath
	}
	if keyPath != "" {
		c.HostCertKeyPath = keyPath
	}
}

func parseEnvVarUint(name string, currentValue uint) uint {
	if v := os.Getenv(name); v != "" {
		val, err := strconv.ParseUint(v, 10, 32)
		if err == nil {
			return uint(val)
		} else {
			fmt.Printf("Error parsing var: %s %v %s\n", name, v, err)
		}
	}
	return currentValue
}

func (c *Config) UpdateFromEnvVars() {
	fmt.Println("Updating config from environment variables...")
	if v := os.Getenv("MILTON_WRITE_URL"); v != "" {
		c.WriteUrl = v
	}
	c.WriteInterval = parseEnvVarUint("MILTON_WRITE_INTERVAL", c.WriteInterval)
	if v := os.Getenv("MILTON_HOST_CERT"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("MILTON_HOST_KEY"); v != "" {
		c.HostCertKeyPath = v
	}
	c.CollectInterval = parseEnvVarUint("MILTON_COLLECT_INTERVAL", c.CollectInterval)
	c.LabelRefreshInterval = parseEnvVarUint("MILTON_LABEL_REFRESH_INTERVAL", c.LabelRefreshInterval)
	c.WriteRetryAttempts = parseEnvVarUint("MILTON_WRITE_RETRY_ATTEMPTS", c.WriteRetryAttempts)
	c.WriteRetryInterval = parseEnvVarUint("MILTON_WRITE_RETRY_INTERVAL", c.WriteRetryInterval)
}
