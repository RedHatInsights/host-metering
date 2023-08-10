package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultConfigPath           = "/etc/milton.conf"
	DefaultWriteUrl             = "http://localhost:9090/api/v1/write"
	DefaultWriteInterval        = 600
	DefaultCertPath             = "/etc/pki/consumer/cert.pem"
	DefaultKeyPath              = "/etc/pki/consumer/key.pem"
	DefaultCollectInterval      = 0
	DefaultLabelRefreshInterval = 86400
	DefaultWriteRetryAttempts   = 8
	DefaultWriteRetryMinInt     = 1
	DefaultWriteRetryMaxInt     = 10
)

type Config struct {
	WriteUrl             string
	WriteInterval        uint // in seconds
	CollectInterval      uint // in seconds
	LabelRefreshInterval uint // in seconds
	HostCertPath         string
	HostCertKeyPath      string
	WriteRetryAttempts   uint
	WriteRetryMinInt     uint // in seconds
	WriteRetryMaxInt     uint // in seconds
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
	fmt.Println("  WriteRetryMinInt: ", c.WriteRetryMinInt)
	fmt.Println("  WriteRetryMaxInt: ", c.WriteRetryMaxInt)
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
	c.WriteRetryMinInt = parseEnvVarUint("MILTON_WRITE_RETRY_MIN_INT", c.WriteRetryMinInt)
	c.WriteRetryMaxInt = parseEnvVarUint("MILTON_WRITE_RETRY_MAX_INT", c.WriteRetryMaxInt)
}

func parseConfigUint(name string, value string, currentValue uint) uint {
	val, err := strconv.ParseUint(value, 10, 32)
	if err == nil {
		return uint(val)
	} else {
		fmt.Printf("Error parsing var: %s %v %s\n", "write_interval", value, err)
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
	if v, ok := config["milton"]["write_interval"]; ok {
		c.WriteInterval = parseConfigUint("write_interval", v, c.WriteInterval)
	}
	if v, ok := config["milton"]["cert_path"]; ok {
		c.HostCertPath = v
	}
	if v, ok := config["milton"]["key_path"]; ok {
		c.HostCertKeyPath = v
	}
	if v, ok := config["milton"]["collect_interval"]; ok {
		c.CollectInterval = parseConfigUint("collect_interval", v, c.CollectInterval)
	}
	if v, ok := config["milton"]["label_refresh_interval"]; ok {
		c.LabelRefreshInterval = parseConfigUint("label_refresh_interval", v, c.LabelRefreshInterval)
	}
	if v, ok := config["milton"]["write_retry_attempts"]; ok {
		c.WriteRetryAttempts = parseConfigUint("write_retry_attempts", v, c.WriteRetryAttempts)
	}
	if v, ok := config["milton"]["write_retry_min_int"]; ok {
		c.WriteRetryMinInt = parseConfigUint("write_retry_min_int", v, c.WriteRetryMinInt)
	}
	if v, ok := config["milton"]["write_retry_max_int"]; ok {
		c.WriteRetryMaxInt = parseConfigUint("write_retry_max_int", v, c.WriteRetryMaxInt)
	}
}
