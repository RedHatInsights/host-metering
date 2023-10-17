package config

import (
	"fmt"
	"time"
)

type Validator interface {
	Validate() error
}

type ConfigValidator struct {
	config *Config
}

func NewConfigValidator(config *Config) *ConfigValidator {
	return &ConfigValidator{config}
}

func (cv *ConfigValidator) Validate() error {
	c := cv.config

	if c.WriteUrl == "" {
		return fmt.Errorf("WriteURL must be defined")
	}

	if c.WriteInterval <= time.Duration(c.WriteRetryAttempts)*(c.WriteRetryMaxInt+c.WriteTimeout) {
		return fmt.Errorf("WriteInterval must be bigger than WriteRetryAttempts * ( WriteRetryMaxInt + WriteTimeout )")
	}

	if c.WriteRetryMinInt >= c.WriteRetryMaxInt {
		return fmt.Errorf("WriteRetryMinInt must be smaller than WriteRetryMaxInt")
	}

	if c.MetricsWALPath == "" {
		return fmt.Errorf("MetricsWALPath must be defined")
	}

	return nil
}
