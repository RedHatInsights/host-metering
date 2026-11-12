package config

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// The path to the configuration file shipped by the RPM package
// (contrib/rpm/host-metering.spec.in installs it as /etc/host-metering.conf
// via %config(noreplace)).
const shippedConfigFile = "../etc-conf/host-metering.conf"

// The shipped config file is curated: it only advertises user-facing
// options. Every key recognized by Config.UpdateFromConfigFile must be
// classified into exactly one of the sets below, so that adding a parser
// key forces a conscious decision whether to expose it to users or to keep
// it as an internal tuning knob. Hidden keys stay configurable (the parser
// honors them and host-metering.conf(5) documents them); they are simply
// not advertised in the shipped file.
var (
	shippedConfigDocumentedKeys = map[string]bool{
		"write_url":     true,
		"send_hostname": true,
		"log_level":     true,
		"log_path":      true,
	}

	shippedConfigHiddenKeys = map[string]bool{
		"write_interval_sec":         true,
		"host_cert_path":             true,
		"host_cert_key_path":         true,
		"collect_interval_sec":       true,
		"label_refresh_interval_sec": true,
		"write_retry_attempts":       true,
		"write_retry_min_int_sec":    true,
		"write_retry_max_int_sec":    true,
		"write_timeout_sec":          true,
		"metrics_max_age_sec":        true,
		"metrics_wal_path":           true,
		"instance_id":                true,
	}
)

// TestShippedConfigFile verifies the configuration file shipped with the RPM:
//   - it parses without errors,
//   - it documents exactly the curated set of user-facing keys, and every
//     documented key is recognized by the parser (the INI parser silently
//     ignores unknown keys, so a typo would otherwise go unnoticed),
//   - every key the parser recognizes is classified, and every classified
//     key is actually recognized by the parser,
//   - the value documented for every curated option equals the compiled-in
//     default, as the file's header claims,
//   - applying it yields a config identical to the compiled-in defaults,
//     i.e. the file is effectively all comments and never overrides defaults.
func TestShippedConfigFile(t *testing.T) {
	if _, err := os.Stat(shippedConfigFile); err != nil {
		t.Fatalf("shipped config file %s not found: %v", shippedConfigFile, err)
	}

	// The two key sets must be disjoint.
	for key := range shippedConfigHiddenKeys {
		if shippedConfigDocumentedKeys[key] {
			t.Fatalf("key %q is classified as both documented and hidden", key)
		}
	}

	// Documented keys and their advertised values: parse "#key = value"
	// comment lines.
	documentedKeys := make(map[string]bool)
	documentedDefaults := make(map[string]string)
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
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" || strings.ContainsAny(key, " \t") {
			// Not a "key = value" line (e.g. free-form comment text).
			continue
		}
		if !shippedConfigDocumentedKeys[key] {
			t.Errorf("shipped config file documents key %q, which is not part of the curated user-facing set (typo/drift, or the key needs to be classified in this test)", key)
		}
		documentedKeys[key] = true
		if value := strings.TrimSpace(parts[1]); value != "" {
			documentedDefaults[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to read shipped config file: %v", err)
	}

	if len(documentedKeys) == 0 {
		t.Fatal("no keys documented in the shipped config file; the parsing of documented keys is probably broken")
	}

	// The shipped file must document exactly the curated set.
	for key := range shippedConfigDocumentedKeys {
		if !documentedKeys[key] {
			t.Errorf("key %q is curated as user-facing but not documented in the shipped config file", key)
		}
	}

	// Every key the parser recognizes must be classified, and every
	// classified key must be recognized: additions to
	// UpdateFromConfigFile must not drift silently.
	parserKeys := parserConfigKeys(t)
	for key := range parserKeys {
		if !shippedConfigDocumentedKeys[key] && !shippedConfigHiddenKeys[key] {
			t.Errorf("parser recognizes key %q which is not classified in this test; decide whether to expose it and classify it", key)
		}
	}
	for key := range shippedConfigDocumentedKeys {
		if !parserKeys[key] {
			t.Errorf("key %q is classified as documented but the parser does not recognize it", key)
		}
	}
	for key := range shippedConfigHiddenKeys {
		if !parserKeys[key] {
			t.Errorf("key %q is classified as hidden but the parser does not recognize it", key)
		}
	}

	// Every classified key must actually be recognized by the parser. The
	// parser silently ignores unknown keys, so each key is verified
	// empirically as well: applying a config file that sets it to a
	// non-default value must change the resulting configuration.
	defaults := NewConfig()
	for key := range shippedConfigDocumentedKeys {
		checkRecognizedKey(t, key, defaults)
	}
	for key := range shippedConfigHiddenKeys {
		checkRecognizedKey(t, key, defaults)
	}

	// The value advertised for every curated option must equal the
	// compiled-in default, as the file's header claims ("Values below
	// match the compiled-in defaults"): applying a one-key file with the
	// advertised value must not change anything. An empty advertised
	// value (log_path) is the compiled-in default and is skipped by the
	// scanner.
	for key, value := range documentedDefaults {
		path := filepath.Join(t.TempDir(), "host-metering.conf")
		createConfigFile(t, path, "[host-metering]\n"+key+" = "+value+"\n")
		c := NewConfig()
		if err := c.UpdateFromConfigFile(path); err != nil {
			t.Errorf("the value documented for %q does not parse: %v", key, err)
			continue
		}
		if !reflect.DeepEqual(c, defaults) {
			t.Errorf("the value documented for %q (%s) does not match the compiled-in default", key, value)
		}
	}

	// The file must parse without errors, and applying it must yield the
	// compiled-in defaults: the file is effectively all comments and must
	// never override a default value. TestDefaultConfig pins the exact
	// defaults; the struct comparison asserts the property under test
	// here without a third copy of the expected dump.
	c := NewConfig()
	if err := c.UpdateFromConfigFile(shippedConfigFile); err != nil {
		t.Fatalf("UpdateFromConfigFile on the shipped file returned an error: %v", err)
	}
	if !reflect.DeepEqual(c, defaults) {
		t.Fatalf("shipped config file overrides compiled-in defaults: %+v", c)
	}
}

// parserConfigKeys returns the INI keys Config.UpdateFromConfigFile reads,
// extracted from the source with go/ast: the hand-written key lookups have
// no runtime enumeration, so the source is the only registry. The keys are
// the string literals used as the inner index of the config[section]["key"]
// expressions in UpdateFromConfigFile's body.
func parserConfigKeys(t *testing.T) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "config.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse config.go: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "UpdateFromConfigFile" {
			fn = fd
			break
		}
	}
	if fn == nil {
		t.Fatal("UpdateFromConfigFile not found in config.go")
	}

	keys := make(map[string]bool)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		index, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		lit, ok := index.Index.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		key, err := strconv.Unquote(lit.Value)
		if err != nil || key == "" || key == "host-metering" {
			return true
		}
		keys[key] = true
		return true
	})
	if len(keys) == 0 {
		t.Fatal("no config keys found in UpdateFromConfigFile; its shape changed, update this test")
	}
	return keys
}

// checkRecognizedKey applies a one-key config file setting key to a value
// that differs from every compiled-in default, and reports an error when
// the resulting configuration is unchanged, i.e. when UpdateFromConfigFile
// silently ignored the key.
func checkRecognizedKey(t *testing.T, key string, defaults *Config) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "host-metering.conf")
	createConfigFile(t, path, "[host-metering]\n"+key+" = 12345\n")

	c := NewConfig()
	if err := c.UpdateFromConfigFile(path); err != nil {
		t.Errorf("key %q: UpdateFromConfigFile returned an error: %v", key, err)
		return
	}
	if reflect.DeepEqual(c, defaults) {
		t.Errorf("key %q is not recognized by UpdateFromConfigFile (config unchanged); update the classification in shipped_conf_test.go", key)
	}
}
