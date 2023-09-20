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
	DefaultMetricsMaxAgeSec        = 5400
	DefaultMetricsWALPath          = "/var/run/milton/metrics"
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
	MetricsMaxAgeSec        uint // in seconds
	MetricsWALPath          string
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
		MetricsMaxAgeSec:        DefaultMetricsMaxAgeSec,
		MetricsWALPath:          DefaultMetricsWALPath,
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
			fmt.Sprintf("  MetricsMaxAgeSec: %d", c.MetricsMaxAgeSec),
			fmt.Sprintf("  MetricsWALPath: %s", c.MetricsWALPath),
			fmt.Sprintf("  LogLevel: %s", c.LogLevel),
			fmt.Sprintf("  LogPath: %s", c.LogPath),
		}, "\n")
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

func (c *Config) UpdateFromEnvVars() error {
	var err error
	var multiError MultiError

	if v := os.Getenv("MILTON_WRITE_URL"); v != "" {
		c.WriteUrl = v
	}

	c.WriteIntervalSec, err = parseEnvVarUint("MILTON_WRITE_INTERVAL_SEC", c.WriteIntervalSec)
	multiError.Add(err)

	if v := os.Getenv("MILTON_HOST_CERT"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("MILTON_HOST_KEY"); v != "" {
		c.HostCertKeyPath = v
	}
	c.CollectIntervalSec, err = parseEnvVarUint("MILTON_COLLECT_INTERVAL_SEC", c.CollectIntervalSec)
	multiError.Add(err)

	c.LabelRefreshIntervalSec, err = parseEnvVarUint("MILTON_LABEL_REFRESH_INTERVAL_SEC", c.LabelRefreshIntervalSec)
	multiError.Add(err)

	c.WriteRetryAttempts, err = parseEnvVarUint("MILTON_WRITE_RETRY_ATTEMPTS", c.WriteRetryAttempts)
	multiError.Add(err)

	c.WriteRetryMinIntSec, err = parseEnvVarUint("MILTON_WRITE_RETRY_MIN_INT_SEC", c.WriteRetryMinIntSec)
	multiError.Add(err)

	c.WriteRetryMaxIntSec, err = parseEnvVarUint("MILTON_WRITE_RETRY_MAX_INT_SEC", c.WriteRetryMaxIntSec)
	multiError.Add(err)

	c.MetricsMaxAgeSec, err = parseEnvVarUint("MILTON_METRICS_MAX_AGE_SEC", c.MetricsMaxAgeSec)
	multiError.Add(err)

	if v := os.Getenv("MILTON_METRICS_WAL_PATH"); v != "" {
		c.MetricsWALPath = v
	}
	if v := os.Getenv("MILTON_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("MILTON_LOG_PATH"); v != "" {
		c.LogPath = v
	}
	return multiError.ErrorOrNil()
}

func parseConfigUint(name string, value string, currentValue uint) (uint, error) {
	val, err := strconv.ParseUint(value, 10, 32)
	if err == nil {
		return uint(val), nil
	} else {
		return currentValue, fmt.Errorf("error parsing var: %s %v %w", name, value, err)
	}
}

func (c *Config) UpdateFromConfigFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no config file at %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening config file: %s", err.Error())
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
		return fmt.Errorf("error reading config file: %s", err.Error())
	}

	// Update config from parsed INI file
	var multiError MultiError

	if v, ok := config["milton"]["write_url"]; ok {
		c.WriteUrl = v
	}
	if v, ok := config["milton"]["write_interval_sec"]; ok {
		c.WriteIntervalSec, err = parseConfigUint("write_interval_sec", v, c.WriteIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval_sec"]; ok {
		c.CollectIntervalSec, err = parseConfigUint("collect_interval_sec", v, c.CollectIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["label_refresh_interval_sec"]; ok {
		c.LabelRefreshIntervalSec, err = parseConfigUint("label_refresh_interval_sec", v, c.LabelRefreshIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts, err = parseConfigUint("write_retry_attempts", v, c.WriteRetryAttempts)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_min_int_sec"]; ok {
		c.WriteRetryMinIntSec, err = parseConfigUint("write_retry_min_int_sec", v, c.WriteRetryMinIntSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_max_int_sec"]; ok {
		c.WriteRetryMaxIntSec, err = parseConfigUint("write_retry_max_int_sec", v, c.WriteRetryMaxIntSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["metrics_max_age_sec"]; ok {
		c.MetricsMaxAgeSec, err = parseConfigUint("metrics_max_age_sec", v, c.MetricsMaxAgeSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["metrics_wal_path"]; ok {
		c.MetricsWALPath = v
	}
	if v, ok := config["milton"]["log_level"]; ok {
		c.LogLevel = v
	}
	if v, ok := config["milton"]["log_path"]; ok {
		c.LogLevel = v
	}

	return multiError.ErrorOrNil()
}

type MultiError struct {
	errors []error
}

func (e *MultiError) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *MultiError) Error() string {
	var msg strings.Builder
	msg.WriteString("multiple errors occurred:")

	for _, err := range e.errors {
		msg.WriteString("\n")
		msg.WriteString(err.Error())
	}

	return msg.String()
}

func (e *MultiError) ErrorOrNil() error {
	if len(e.errors) == 0 {
		return nil
	} else {
		return e
	}
}
