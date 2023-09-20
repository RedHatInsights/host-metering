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

func (c *Config) UpdateFromEnvVars() error {
	var err error
	var multiError MultiError

	if v := os.Getenv("MILTON_WRITE_URL"); v != "" {
		c.WriteUrl = v
	}
	if v := os.Getenv("MILTON_WRITE_INTERVAL_SEC"); v != "" {
		c.WriteIntervalSec, err = parseUint("MILTON_WRITE_INTERVAL_SEC", v, c.WriteIntervalSec)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_HOST_CERT_PATH"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("MILTON_HOST_CERT_KEY_PATH"); v != "" {
		c.HostCertKeyPath = v
	}
	if v := os.Getenv("MILTON_COLLECT_INTERVAL_SEC"); v != "" {
		c.CollectIntervalSec, err = parseUint("MILTON_COLLECT_INTERVAL_SEC", v, c.CollectIntervalSec)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_LABEL_REFRESH_INTERVAL_SEC"); v != "" {
		c.LabelRefreshIntervalSec, err = parseUint("MILTON_LABEL_REFRESH_INTERVAL_SEC", v, c.LabelRefreshIntervalSec)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_WRITE_RETRY_ATTEMPTS"); v != "" {
		c.WriteRetryAttempts, err = parseUint("MILTON_WRITE_RETRY_ATTEMPTS", v, c.WriteRetryAttempts)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_WRITE_RETRY_MIN_INT_SEC"); v != "" {
		c.WriteRetryMinIntSec, err = parseUint("MILTON_WRITE_RETRY_MIN_INT_SEC", v, c.WriteRetryMinIntSec)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_WRITE_RETRY_MAX_INT_SEC"); v != "" {
		c.WriteRetryMaxIntSec, err = parseUint("MILTON_WRITE_RETRY_MAX_INT_SEC", v, c.WriteRetryMaxIntSec)
		multiError.Add(err)
	}
	if v := os.Getenv("MILTON_METRICS_MAX_AGE_SEC"); v != "" {
		c.MetricsMaxAgeSec, err = parseUint("MILTON_METRICS_MAX_AGE_SEC", v, c.MetricsMaxAgeSec)
		multiError.Add(err)
	}
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

type INIConfig map[string]map[string]string

func (config INIConfig) parseFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no config file at %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening config file: %s", err.Error())
	}
	defer file.Close()

	// Parse INI file
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

	return nil
}

func (c *Config) UpdateFromConfigFile(path string) error {
	// Parse the INI file.
	config := INIConfig{}

	err := config.parseFile(path)
	if err != nil {
		return err
	}

	// Update config from parsed INI file
	var multiError MultiError

	if v, ok := config["milton"]["write_url"]; ok {
		c.WriteUrl = v
	}
	if v, ok := config["milton"]["write_interval_sec"]; ok {
		c.WriteIntervalSec, err = parseUint("write_interval_sec", v, c.WriteIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["host_cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["host_cert_key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval_sec"]; ok {
		c.CollectIntervalSec, err = parseUint("collect_interval_sec", v, c.CollectIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["label_refresh_interval_sec"]; ok {
		c.LabelRefreshIntervalSec, err = parseUint("label_refresh_interval_sec", v, c.LabelRefreshIntervalSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts, err = parseUint("write_retry_attempts", v, c.WriteRetryAttempts)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_min_int_sec"]; ok {
		c.WriteRetryMinIntSec, err = parseUint("write_retry_min_int_sec", v, c.WriteRetryMinIntSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_max_int_sec"]; ok {
		c.WriteRetryMaxIntSec, err = parseUint("write_retry_max_int_sec", v, c.WriteRetryMaxIntSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["metrics_max_age_sec"]; ok {
		c.MetricsMaxAgeSec, err = parseUint("metrics_max_age_sec", v, c.MetricsMaxAgeSec)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["metrics_wal_path"]; ok {
		c.MetricsWALPath = v
	}
	if v, ok := config["milton"]["log_level"]; ok {
		c.LogLevel = v
	}
	if v, ok := config["milton"]["log_path"]; ok {
		c.LogPath = v
	}

	return multiError.ErrorOrNil()
}

func parseUint(name string, value string, defaultValue uint) (uint, error) {
	parsedValue, err := strconv.ParseUint(value, 10, 32)

	if err != nil {
		return defaultValue, fmt.Errorf("invalid value of '%s': %v", name, err.Error())
	}

	return uint(parsedValue), nil
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
