package logger

import (
	"os"
	"strings"
	"testing"
)

// Test that logger global functions won't crash even if the logger is not initialized.
func TestLoggerGlobalFunctions(t *testing.T) {
	clearLogger()
	Error("Error Test")
	clearLogger()
	Errorf("Errorf %s", "Test")
	clearLogger()
	Errorln("Errorln Test")

	clearLogger()
	Warn("Warn Test")
	clearLogger()
	Warnf("Warnf %s", "test")
	clearLogger()
	Warnln("Warnln Test")

	clearLogger()
	Info("Info Test")
	clearLogger()
	Infof("Infof %s", "test")
	clearLogger()
	Infoln("Infoln Test")

	clearLogger()
	Debug("Debug Test")
	clearLogger()
	Debugf("Debugf Test %s", "test")
	clearLogger()
	Debugln("Debugln Test")
}

// Test initialization of logger with only log level
func TestInitLogger(t *testing.T) {
	InitLogger("", DebugLevel.String())
	if log == nil {
		t.Fatalf("logger is not initialized")
	}
	// Test some usuage
	log.Debug("Debug Test")
}

// Test initialization of the logger which writes to a file.
func TestInitLoggerFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.log"
	InitLogger(path, DebugLevel.String())
	if log == nil {
		t.Fatalf("logger is not initialized")
	}
	testMsg := "Test message"
	// Test some usuage
	log.Debug(testMsg)

	// Check that the file is created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("log file is not created")
	}

	// Check that the file contains the logger message
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the log file")
	}
	if !strings.Contains(string(data), testMsg) {
		t.Fatalf("log file does not contain the logged message")
	}
}

// Test that logger can be raplaced by other implementation of Logger interface.
func TestOverrideLogger(t *testing.T) {
	logger := NewTestLogger()
	OverrideLogger(logger)
	if log != logger {
		t.Fatalf("logger is not overridden")
	}

	currrentLogger := getLogger()
	if currrentLogger != logger {
		t.Fatalf("logger is not overridden")
	}
}

// Test that global logger functions will use the overriden logger.
func TestOverridenLogger(t *testing.T) {
	logger := NewTestLogger()
	OverrideLogger(logger)

	// Test that getting last entry when log is empty returns nil
	entry := logger.GetLastEntry()
	if entry != nil {
		t.Fatalf("unexpected last log entry: %v", entry)
	}

	// Test that the logger can be cleared even when it is empty
	logger.Clear()

	// IsLastEntry should return false when log is empty
	if logger.IsLastEntry(ErrorLevel, "", "Error") {
		t.Fatalf("Unexpected last log entry")
	}

	// Test individual log calls via global functions
	Error("Error Test")
	if !logger.IsLastEntry(ErrorLevel, "Error Test", "Error") {
		t.Fatalf("Error message is not logged")
	}

	Errorf("Errorf %s", "Test")
	if !logger.IsLastEntry(ErrorLevel, "Errorf Test", "Errorf") {
		t.Fatalf("Errorf message is not logged")
	}

	Errorln("Errorln Test")
	if !logger.IsLastEntry(ErrorLevel, "Errorln Test", "Errorln") {
		t.Fatalf("Errorln message is not logged")
	}

	Warn("Warn Test")
	if !logger.IsLastEntry(WarnLevel, "Warn Test", "Warn") {
		t.Fatalf("Warn message is not logged")
	}

	Warnf("Warnf %s", "test")
	if !logger.IsLastEntry(WarnLevel, "Warnf test", "Warnf") {
		t.Fatalf("Warnf message is not logged")
	}

	Warnln("Warnln Test")
	if !logger.IsLastEntry(WarnLevel, "Warnln Test", "Warnln") {
		t.Fatalf("Warnln message is not logged")
	}

	Info("Info Test")
	if !logger.IsLastEntry(InfoLevel, "Info Test", "Info") {
		t.Fatalf("Info message is not logged")
	}

	Infof("Infof %s", "test")
	if !logger.IsLastEntry(InfoLevel, "Infof test", "Infof") {
		t.Fatalf("Infof message is not logged")
	}

	Infoln("Infoln Test")
	if !logger.IsLastEntry(InfoLevel, "Infoln Test", "Infoln") {
		t.Fatalf("Infoln message is not logged")
	}

	Debug("Debug Test")
	if !logger.IsLastEntry(DebugLevel, "Debug Test", "Debug") {
		t.Fatalf("Debug message is not logged")
	}

	Debugf("Debugf Test %s", "test")
	if !logger.IsLastEntry(DebugLevel, "Debugf Test test", "Debugf") {
		t.Fatalf("Debugf message is not logged")
	}

	Debugln("Debugln Test")
	if !logger.IsLastEntry(DebugLevel, "Debugln Test", "Debugln") {
		t.Fatalf("Debugln message is not logged")
	}

	entries := logger.GetEntries()
	if len(entries) != 12 {
		t.Fatalf("unexpected number of log entries: %d", len(entries))
	}

	// Test that clearing
	logger.Clear()
	entries = logger.GetEntries()
	if len(entries) != 0 {
		t.Fatalf("unexpected number of log entries after clear: %d", len(entries))
	}
}

// Helper functions

func clearLogger() {
	log = nil
}
