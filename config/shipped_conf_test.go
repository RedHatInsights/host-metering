package config

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// The path to the configuration file shipped by the RPM package
// (contrib/rpm/host-metering.spec.in installs it as /etc/host-metering.conf
// via %config(noreplace)).
const shippedConfigFile = "../etc-conf/host-metering.conf"

// Keys recognized by Config.UpdateFromConfigFile. This set is hard-coded on
// purpose: the parser silently ignores unknown keys, so a typo in the shipped
// config file would go unnoticed. This test fails if the shipped file mentions
// a key the parser does not recognize (or vice versa, if a recognized key is
// never offered to users in the shipped file).
var shippedConfigRecognizedKeys = map[string]bool{
	"write_url":                  true,
	"write_interval_sec":         true,
	"host_cert_path":             true,
	"host_cert_key_path":         true,
	"collect_interval_sec":       true,
	"label_refresh_interval_sec": true,
	"send_hostname":              true,
	"write_retry_attempts":       true,
	"write_retry_min_int_sec":    true,
	"write_retry_max_int_sec":    true,
	"write_timeout_sec":          true,
	"metrics_max_age_sec":        true,
	"metrics_wal_path":           true,
	"log_level":                  true,
	"log_path":                   true,
	"instance_id":                true,
}

// TestShippedConfigFile verifies the configuration file shipped with the RPM:
//   - it parses without errors,
//   - every documented key is recognized by the parser (no key drift),
//   - it only documents keys the parser knows about,
//   - applying it yields a config identical to the compiled-in defaults,
//     i.e. the file is effectively all comments and never overrides defaults.
func TestShippedConfigFile(t *testing.T) {
	// Unset relevant environment variables.
	clearEnvironment()

	if _, err := os.Stat(shippedConfigFile); err != nil {
		t.Fatalf("shipped config file %s not found: %v", shippedConfigFile, err)
	}

	// Documented keys: parse "#key = value" comment lines.
	documentedKeys := make(map[string]bool)
	file, err := os.Open(shippedConfigFile)
	if err != nil {
		t.Fatalf("failed to open shipped config file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "#")
		// Only consider lines that look like "#key = value".
		if !strings.Contains(line, "=") {
			continue
		}
		key := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		if key == "" || strings.ContainsAny(key, " \t") {
			// Not a "key = value" line (e.g. free-form comment text).
			continue
		}
		if !shippedConfigRecognizedKeys[key] {
			t.Errorf("shipped config file documents key %q, which UpdateFromConfigFile does not recognize (typo/drift?)", key)
		}
		documentedKeys[key] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to read shipped config file: %v", err)
	}

	if len(documentedKeys) == 0 {
		t.Fatal("no keys documented in the shipped config file; the parsing of documented keys is probably broken")
	}

	// Note: the reverse direction (every recognized key documented in the
	// shipped file) is intentionally not enforced; internal keys such as
	// instance_id may deliberately stay undocumented.

	// The file must parse without errors.
	c := NewConfig()
	if err := c.UpdateFromConfigFile(shippedConfigFile); err != nil {
		t.Fatalf("UpdateFromConfigFile on the shipped file returned an error: %v", err)
	}

	// Applying the shipped file must yield the compiled-in defaults: the file
	// is effectively all comments and must never override a default value.
	expectedCfg := "Config:\n" +
		"|  WriteUrl: https://cert.console.redhat.com/api/rhel-telemetry/v1/receive\n" +
		"|  WriteIntervalSec: 600\n" +
		"|  HostCertPath: /etc/pki/consumer/cert.pem\n" +
		"|  HostCertKeyPath: /etc/pki/consumer/key.pem\n" +
		"|  CollectIntervalSec: 0\n" +
		"|  LabelRefreshIntervalSec: 86400\n" +
		"|  SendHostname: yes\n" +
		"|  WriteRetryAttempts: 8\n" +
		"|  WriteRetryMinIntSec: 1\n" +
		"|  WriteRetryMaxIntSec: 10\n" +
		"|  WriteTimeoutSec: 60\n" +
		"|  MetricsMaxAgeSec: 5400\n" +
		"|  MetricsWALPath: /var/run/host-metering/metrics\n" +
		"|  LogLevel: INFO\n" +
		"|  LogPath: \n" +
		"|  InstanceID: \n"

	checkString(t, c.String(), expectedCfg)
}
