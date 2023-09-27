package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultConfigPath           = "/etc/milton.conf"
	DefaultWriteUrl             = "http://localhost:9090/api/v1/write"
	DefaultWriteInterval        = 600 * time.Second
	DefaultCertPath             = "/etc/pki/consumer/cert.pem"
	DefaultKeyPath              = "/etc/pki/consumer/key.pem"
	DefaultCollectInterval      = 0 * time.Second
	DefaultLabelRefreshInterval = 86400 * time.Second
	DefaultWriteRetryAttempts   = 8
	DefaultWriteRetryMinInt     = 1 * time.Second
	DefaultWriteRetryMaxInt     = 10 * time.Second
	DefaultMetricsMaxAge        = 5400 * time.Second
	DefaultMetricsWALPath       = "/var/run/milton/metrics"
	DefaultLogLevel             = "INFO"
	DefaultLogPath              = "" //Default to stderr, will be logged in journal.
)

type Config struct {
	WriteUrl             string
	WriteInterval        time.Duration
	CollectInterval      time.Duration
	LabelRefreshInterval time.Duration
	HostCertPath         string
	HostCertKeyPath      string
	WriteRetryAttempts   uint
	WriteRetryMinInt     time.Duration
	WriteRetryMaxInt     time.Duration
	MetricsMaxAge        time.Duration
	MetricsWALPath       string
	LogLevel             string // one of "ERROR", "WARN", "INFO", "DEBUG", "TRACE"
	LogPath              string
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
		WriteRetryMinInt:     DefaultWriteRetryMinInt,
		WriteRetryMaxInt:     DefaultWriteRetryMaxInt,
		MetricsMaxAge:        DefaultMetricsMaxAge,
		MetricsWALPath:       DefaultMetricsWALPath,
		LogLevel:             DefaultLogLevel,
		LogPath:              DefaultLogPath,
	}
}

func (c *Config) String() string {
	return strings.Join(
		[]string{
			"Config:",
			fmt.Sprintf("  WriteUrl: %s", c.WriteUrl),
			fmt.Sprintf("  WriteIntervalSec: %.0f", c.WriteInterval.Seconds()),
			fmt.Sprintf("  HostCertPath: %s", c.HostCertPath),
			fmt.Sprintf("  HostCertKeyPath: %s", c.HostCertKeyPath),
			fmt.Sprintf("  CollectIntervalSec: %.0f", c.CollectInterval.Seconds()),
			fmt.Sprintf("  LabelRefreshIntervalSec: %.0f", c.LabelRefreshInterval.Seconds()),
			fmt.Sprintf("  WriteRetryAttempts: %d", c.WriteRetryAttempts),
			fmt.Sprintf("  WriteRetryMinIntSec: %.0f", c.WriteRetryMinInt.Seconds()),
			fmt.Sprintf("  WriteRetryMaxIntSec: %.0f", c.WriteRetryMaxInt.Seconds()),
			fmt.Sprintf("  MetricsMaxAgeSec: %.0f", c.MetricsMaxAge.Seconds()),
			fmt.Sprintf("  MetricsWALPath: %s", c.MetricsWALPath),
			fmt.Sprintf("  LogLevel: %s", c.LogLevel),
			fmt.Sprintf("  LogPath: %s", c.LogPath),
		}, "\n")
}

func (c *Config) UpdateFromEnvVars() error {
	var err error
	var multiError MultiError

	if v := os.Getenv("HOST_METERING_WRITE_URL"); v != "" {
		c.WriteUrl = v
	}
	if v := os.Getenv("HOST_METERING_WRITE_INTERVAL_SEC"); v != "" {
		c.WriteInterval, err = parseSeconds("HOST_METERING_WRITE_INTERVAL_SEC", v, c.WriteInterval)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_HOST_CERT_PATH"); v != "" {
		c.HostCertPath = v
	}
	if v := os.Getenv("HOST_METERING_HOST_CERT_KEY_PATH"); v != "" {
		c.HostCertKeyPath = v
	}
	if v := os.Getenv("HOST_METERING_COLLECT_INTERVAL_SEC"); v != "" {
		c.CollectInterval, err = parseSeconds("HOST_METERING_COLLECT_INTERVAL_SEC", v, c.CollectInterval)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC"); v != "" {
		c.LabelRefreshInterval, err = parseSeconds("HOST_METERING_LABEL_REFRESH_INTERVAL_SEC", v, c.LabelRefreshInterval)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_WRITE_RETRY_ATTEMPTS"); v != "" {
		c.WriteRetryAttempts, err = parseUint("HOST_METERING_WRITE_RETRY_ATTEMPTS", v, c.WriteRetryAttempts)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_WRITE_RETRY_MIN_INT_SEC"); v != "" {
		c.WriteRetryMinInt, err = parseSeconds("HOST_METERING_WRITE_RETRY_MIN_INT_SEC", v, c.WriteRetryMinInt)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_WRITE_RETRY_MAX_INT_SEC"); v != "" {
		c.WriteRetryMaxInt, err = parseSeconds("HOST_METERING_WRITE_RETRY_MAX_INT_SEC", v, c.WriteRetryMaxInt)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_METRICS_MAX_AGE_SEC"); v != "" {
		c.MetricsMaxAge, err = parseSeconds("HOST_METERING_METRICS_MAX_AGE_SEC", v, c.MetricsMaxAge)
		multiError.Add(err)
	}
	if v := os.Getenv("HOST_METERING_METRICS_WAL_PATH"); v != "" {
		c.MetricsWALPath = v
	}
	if v := os.Getenv("HOST_METERING_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("HOST_METERING_LOG_PATH"); v != "" {
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
		c.WriteInterval, err = parseSeconds("write_interval_sec", v, c.WriteInterval)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["host_cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["host_cert_key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval_sec"]; ok {
		c.CollectInterval, err = parseSeconds("collect_interval_sec", v, c.CollectInterval)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["label_refresh_interval_sec"]; ok {
		c.LabelRefreshInterval, err = parseSeconds("label_refresh_interval_sec", v, c.LabelRefreshInterval)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts, err = parseUint("write_retry_attempts", v, c.WriteRetryAttempts)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_min_int_sec"]; ok {
		c.WriteRetryMinInt, err = parseSeconds("write_retry_min_int_sec", v, c.WriteRetryMinInt)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["write_retry_max_int_sec"]; ok {
		c.WriteRetryMaxInt, err = parseSeconds("write_retry_max_int_sec", v, c.WriteRetryMaxInt)
		multiError.Add(err)
	}
	if v, ok := config["milton"]["metrics_max_age_sec"]; ok {
		c.MetricsMaxAge, err = parseSeconds("metrics_max_age_sec", v, c.MetricsMaxAge)
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

func parseSeconds(name string, value string, defaultValue time.Duration) (time.Duration, error) {
	parsedValue, err := strconv.ParseUint(value, 10, 32)

	if err != nil {
		return defaultValue, fmt.Errorf("invalid value of '%s': %v", name, err.Error())
	}

	return time.Duration(parsedValue) * time.Second, nil
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
