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
	DefaultLogLevel                = "INFO"
	DefaultLogPath                 = "" //Default to stderr, will be logged in journal.
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
	LogLevel                string // one of "ERROR", "WARN", "INFO", "DEBUG", "TRACE"
	LogPath                 string
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
		LogLevel:                DefaultLogLevel,
		LogPath:                 DefaultLogPath,
	}
}

func (c *Config) String() string {
	return strings.Join(
		[]string{
			"Config:",
			fmt.Sprintf("  WriteUrl: %s", c.WriteUrl),
			fmt.Sprintf("  WriteIntervalSec: %d", c.WriteIntervalSec),
			fmt.Sprintf("  HostCertPath: %s", c.HostCertPath),
			fmt.Sprintf("  HostCertKeyPath: %s", c.HostCertKeyPath),
			fmt.Sprintf("  CollectIntervalSec: %d", c.CollectIntervalSec),
			fmt.Sprintf("  LabelRefreshIntervalSec: %d", c.LabelRefreshIntervalSec),
			fmt.Sprintf("  WriteRetryAttempts: %d", c.WriteRetryAttempts),
			fmt.Sprintf("  WriteRetryMinIntSec: %d", c.WriteRetryMinIntSec),
			fmt.Sprintf("  WriteRetryMaxIntSec: %d", c.WriteRetryMaxIntSec),
			fmt.Sprintf("  CpuCachePath: %s", c.CpuCachePath),
			fmt.Sprintf("  LogLevel: %s", c.LogLevel),
			fmt.Sprintf("  LogPath: %s", c.LogPath),
		}, "\n")
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

func parseEnvVarUint(name string, currentValue uint) (uint, error) {
	if v := os.Getenv(name); v != "" {
		val, err := strconv.ParseUint(v, 10, 32)
		if err == nil {
			return uint(val), nil
		} else {
			return currentValue, fmt.Errorf("error parsing var: %s %v %w", name, v, err)
		}
	}
	return currentValue, nil
}

func (c *Config) UpdateFromEnvVars() string {
	var errors strings.Builder
	if v := os.Getenv("MILTON_WRITE_URL"); v != "" {
		c.WriteUrl = v
	}
	var err error
	c.WriteIntervalSec, err = parseEnvVarUint("MILTON_WRITE_INTERVAL_SEC", c.WriteIntervalSec)
	if err != nil {
		errors.WriteString(err.Error())
	}

	if v := os.Getenv("MILTON_HOST_CERT"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("MILTON_HOST_KEY"); v != "" {
		c.HostCertKeyPath = v
	}
	c.CollectIntervalSec, err = parseEnvVarUint("MILTON_COLLECT_INTERVAL_SEC", c.CollectIntervalSec)
	if err != nil {
		errors.WriteString(err.Error())
	}
	c.LabelRefreshIntervalSec, err = parseEnvVarUint("MILTON_LABEL_REFRESH_INTERVAL_SEC", c.LabelRefreshIntervalSec)
	if err != nil {
		errors.WriteString(err.Error())
	}
	c.WriteRetryAttempts, err = parseEnvVarUint("MILTON_WRITE_RETRY_ATTEMPTS", c.WriteRetryAttempts)
	if err != nil {
		errors.WriteString(err.Error())
	}
	c.WriteRetryMinIntSec, err = parseEnvVarUint("MILTON_WRITE_RETRY_MIN_INT_SEC", c.WriteRetryMinIntSec)
	if err != nil {
		errors.WriteString(err.Error())
	}
	c.WriteRetryMaxIntSec, err = parseEnvVarUint("MILTON_WRITE_RETRY_MAX_INT_SEC", c.WriteRetryMaxIntSec)
	if err != nil {
		errors.WriteString(err.Error())
	}

	if v := os.Getenv("MILTON_CPU_CACHE_PATH"); v != "" {
		c.CpuCachePath = v
	}
	if v := os.Getenv("MILTON_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("MILTON_LOG_PATH"); v != "" {
		c.LogPath = v
	}
	return errors.String()
}

func parseConfigUint(name string, value string, currentValue uint) (uint, error) {
	val, err := strconv.ParseUint(value, 10, 32)
	if err == nil {
		return uint(val), nil
	} else {
		return currentValue, fmt.Errorf("error parsing var: %s %v %w", name, value, err)
	}
}

func (c *Config) UpdateFromConfigFile(path string) string {
	if _, err := os.Stat(path); err != nil {
		return fmt.Sprintf("Config file %s doesn't exist, skipping...\n", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("Error opening config file: %s\n", err.Error())
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
		return fmt.Sprintf("Error reading config file: %s", err.Error())
	}

	var errors strings.Builder
	// Update config from parsed INI file
	if v, ok := config["milton"]["write_url"]; ok {
		c.WriteUrl = v
	}
	if v, ok := config["milton"]["write_interval_sec"]; ok {
		c.WriteIntervalSec, err = parseConfigUint("write_interval_sec", v, c.WriteIntervalSec)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval_sec"]; ok {
		c.CollectIntervalSec, err = parseConfigUint("collect_interval_sec", v, c.CollectIntervalSec)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["label_refresh_interval_sec"]; ok {
		c.LabelRefreshIntervalSec, err = parseConfigUint("label_refresh_interval_sec", v, c.LabelRefreshIntervalSec)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts, err = parseConfigUint("write_retry_attempts", v, c.WriteRetryAttempts)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["write_retry_min_int_sec"]; ok {
		c.WriteRetryMinIntSec, err = parseConfigUint("write_retry_min_int_sec", v, c.WriteRetryMinIntSec)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["write_retry_max_int_sec"]; ok {
		c.WriteRetryMaxIntSec, err = parseConfigUint("write_retry_max_int_sec", v, c.WriteRetryMaxIntSec)
		if err != nil {
			errors.WriteString(err.Error())
		}
	}
	if v, ok := config["milton"]["cpu_cache_path"]; ok {
		c.CpuCachePath = v
	}
	if v, ok := config["milton"]["log_level"]; ok {
		c.LogLevel = v
	}
	if v, ok := config["milton"]["log_path"]; ok {
		c.LogLevel = v
	}

	return errors.String()
}
