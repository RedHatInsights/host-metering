package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidator(t *testing.T) {
	t.Run("Validate Config", func(t *testing.T) {
		t.Run("WriteURL must be defined", func(t *testing.T) {
			// given
			c := NewConfig()
			c.WriteUrl = ""
			cv := NewConfigValidator(c)

			// when
			err := cv.Validate()

			// then
			expectErrorContains(t, err, "WriteURL must be defined")
		})

		t.Run("overlapping requests", func(t *testing.T) {
			// given
			c := NewConfig()
			c.WriteInterval = 80 * time.Second
			c.WriteRetryAttempts = 8
			c.WriteRetryMaxInt = 10 * time.Second
			c.WriteTimeout = 60 * time.Second
			cv := NewConfigValidator(c)

			// when
			err := cv.Validate()

			// then
			expectErrorContains(t, err, "WriteInterval must be bigger than WriteRetryAttempts * ( WriteRetryMaxInt + WriteTimeout )")
		})

		t.Run("WriteRetryMinInt to be smaller than WriteRetryMaxInt", func(t *testing.T) {
			// given
			c := NewConfig()
			c.WriteRetryMinInt = 10 * time.Second
			c.WriteRetryMaxInt = 1 * time.Second
			cv := NewConfigValidator(c)

			// when
			err := cv.Validate()

			// then
			expectErrorContains(t, err, "WriteRetryMinInt must be smaller than WriteRetryMaxInt")
		})

		t.Run("MetricsWALPath must be defined", func(t *testing.T) {
			// given
			c := NewConfig()
			c.MetricsWALPath = ""
			cv := NewConfigValidator(c)

			// when
			err := cv.Validate()

			// then
			expectErrorContains(t, err, "MetricsWALPath must be defined")
		})

		t.Run("default config should be valid", func(t *testing.T) {
			// given
			c := NewConfig()
			cv := NewConfigValidator(c)

			// when
			err := cv.Validate()

			// then
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	})
}

// Helpers

func expectError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func expectErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	expectError(t, err)
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error to contain '%s', got '%s'", expected, err.Error())
	}
}
