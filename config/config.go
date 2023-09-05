package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultConfigPath              = "/etc/milton.conf"
	DefaultWriteUrl                = "http://localhost:9090/api/v1/write"
	DefaultWriteIntervalSec        = 600
	DefaultCertPath                = "/etc/pki/consumer/cert.pem"
	DefaultKeyPath                 = "/etc/pki/consumer/key.pem"
	DefaultCollectIntervalSec      = 0
	DefaultLabelRefreshIntervalSec = 86400
	DefaultWriteRetryAttempts      = 8
	DefaultWriteRetryMinIntSec     = 1
	DefaultWriteRetryMaxIntSec     = 10
	DefaultCpuCachePath            = "/var/run/milton/cpucache"
)

type Config struct {
	WriteUrl                string
	WriteIntervalSec        uint // in seconds
	CollectIntervalSec      uint // in seconds
	LabelRefreshIntervalSec uint // in seconds
	HostCertPath            string
	HostCertKeyPath         string
	WriteRetryAttempts      uint
	WriteRetryMinIntSec     uint // in seconds
	WriteRetryMaxIntSec     uint // in seconds
	CpuCachePath            string
}

func NewConfig() *Config {
	return &Config{
		WriteUrl:                DefaultWriteUrl,
		WriteIntervalSec:        DefaultWriteIntervalSec,
		HostCertPath:            DefaultCertPath,
		HostCertKeyPath:         DefaultKeyPath,
		CollectIntervalSec:      DefaultCollectIntervalSec,
		LabelRefreshIntervalSec: DefaultLabelRefreshIntervalSec,
		WriteRetryAttempts:      DefaultWriteRetryAttempts,
		WriteRetryMinIntSec:     DefaultWriteRetryMinIntSec,
		WriteRetryMaxIntSec:     DefaultWriteRetryMaxIntSec,
		CpuCachePath:            DefaultCpuCachePath,
	}
}

func (c *Config) Print() {
	fmt.Println("Config:")
	fmt.Println("  WriteUrl: ", c.WriteUrl)
	fmt.Println("  WriteIntervalSec: ", c.WriteIntervalSec)
	fmt.Println("  HostCertPath: ", c.HostCertPath)
	fmt.Println("  HostCertKeyPath: ", c.HostCertKeyPath)
	fmt.Println("  CollectIntervalSec: ", c.CollectIntervalSec)
	fmt.Println("  LabelRefreshIntervalSec: ", c.LabelRefreshIntervalSec)
	fmt.Println("  WriteRetryAttempts: ", c.WriteRetryAttempts)
	fmt.Println("  WriteRetryMinIntSec: ", c.WriteRetryMinIntSec)
	fmt.Println("  WriteRetryMaxIntSec: ", c.WriteRetryMaxIntSec)
	fmt.Println("  CpuCachePath: ", c.CpuCachePath)
}

func (c *Config) UpdateFromCliOptions(writeUrl string, writeIntervalSec uint, certPath string, keyPath string) {
	if writeUrl != "" {
		c.WriteUrl = writeUrl
	}
	if writeIntervalSec != 0 {
		c.WriteIntervalSec = writeIntervalSec
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
	c.WriteIntervalSec = parseEnvVarUint("MILTON_WRITE_INTERVAL_SEC", c.WriteIntervalSec)
	if v := os.Getenv("MILTON_HOST_CERT"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("MILTON_HOST_KEY"); v != "" {
		c.HostCertKeyPath = v
	}
	c.CollectIntervalSec = parseEnvVarUint("MILTON_COLLECT_INTERVAL_SEC", c.CollectIntervalSec)
	c.LabelRefreshIntervalSec = parseEnvVarUint("MILTON_LABEL_REFRESH_INTERVAL_SEC", c.LabelRefreshIntervalSec)
	c.WriteRetryAttempts = parseEnvVarUint("MILTON_WRITE_RETRY_ATTEMPTS", c.WriteRetryAttempts)
	c.WriteRetryMinIntSec = parseEnvVarUint("MILTON_WRITE_RETRY_MIN_INT_SEC", c.WriteRetryMinIntSec)
	c.WriteRetryMaxIntSec = parseEnvVarUint("MILTON_WRITE_RETRY_MAX_INT_SEC", c.WriteRetryMaxIntSec)
	if v := os.Getenv("MILTON_CPU_CACHE_PATH"); v != "" {
		c.CpuCachePath = v
	}
}

func parseConfigUint(name string, value string, currentValue uint) uint {
	val, err := strconv.ParseUint(value, 10, 32)
	if err == nil {
		return uint(val)
	} else {
		fmt.Printf("Error parsing var: %s %v %s\n", name, value, err)
	}
	return currentValue
}

func (c *Config) UpdateFromConfigFile(path string) {
	fmt.Println("Updating config from config file...")

	if _, err := os.Stat(path); err != nil {
		fmt.Println("Config file ", path, " doesn't exist, skipping...")
		return
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return
	}
	defer file.Close()

	// Parse INI file
	config := make(map[string]map[string]string)
	currentSection := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this line is a section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			config[currentSection] = make(map[string]string)
		} else if currentSection != "" {
			// Parse key-value pairs
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				config[currentSection][key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}

	// Update config from parsed INI file
	if v, ok := config["milton"]["write_url"]; ok {
		c.WriteUrl = v
	}
	if v, ok := config["milton"]["write_interval_sec"]; ok {
		c.WriteIntervalSec = parseConfigUint("write_interval_sec", v, c.WriteIntervalSec)
	}
	if v, ok := config["milton"]["cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval_sec"]; ok {
		c.CollectIntervalSec = parseConfigUint("collect_interval_sec", v, c.CollectIntervalSec)
	}
	if v, ok := config["milton"]["label_refresh_interval_sec"]; ok {
		c.LabelRefreshIntervalSec = parseConfigUint("label_refresh_interval_sec", v, c.LabelRefreshIntervalSec)
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts = parseConfigUint("write_retry_attempts", v, c.WriteRetryAttempts)
	}
	if v, ok := config["milton"]["write_retry_min_int_sec"]; ok {
		c.WriteRetryMinIntSec = parseConfigUint("write_retry_min_int_sec", v, c.WriteRetryMinIntSec)
	}
	if v, ok := config["milton"]["write_retry_max_int_sec"]; ok {
		c.WriteRetryMaxIntSec = parseConfigUint("write_retry_max_int_sec", v, c.WriteRetryMaxIntSec)
	}
	if v, ok := config["milton"]["cpu_cache_path"]; ok {
		c.CpuCachePath = v
	}
}
