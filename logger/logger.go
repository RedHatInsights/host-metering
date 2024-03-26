package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	logrus "github.com/sirupsen/logrus"
)

const (
	DebugLevel = logrus.DebugLevel
	InfoLevel  = logrus.InfoLevel
	WarnLevel  = logrus.WarnLevel
	ErrorLevel = logrus.ErrorLevel
)

type Logger logrus.FieldLogger

type CustomFormatter struct{}

func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	message := entry.Message
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}
	msg := fmt.Sprintf("%s %s", entry.Time.Format("2006/01/02 15:04:05"), message)
	return []byte(msg), nil
}

func InitDefaultLogger() Logger {
	logger := logrus.New()
	logger.SetFormatter(&CustomFormatter{})
	return logger
}

func InitLogger(file string, level string) error {
	logLevel, err := logrus.ParseLevel(level)

	if err != nil {
		return err
	}

	logFile := os.Stderr
	if file != "" {
		logFile, err = os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	}
	if err != nil {
		return err
	}

	log = &logrus.Logger{
		Out:       logFile,
		Formatter: &CustomFormatter{},
		Level:     logLevel,
	}

	return nil
}

// Inject a predefined logger instance, will be used for testing.
func OverrideLogger(newInstance Logger) {
	log = newInstance
}

var log Logger = nil

func getLogger() Logger {
	if log == nil {
		log = InitDefaultLogger()
	}
	return log
}

// Error prints to the logger if level is at least LevelError. Arguments are
// handled in the manner of fmt.Print.
func Error(v ...interface{}) {
	getLogger().Error(v...)
}

// Errorf prints to the logger if level is at least LevelError. Arguments are
// handled in the manner of fmt.Printf.
func Errorf(format string, v ...interface{}) {
	getLogger().Errorf(format, v...)
}

// Errorln prints to the logger if level is at least LevelError. Arguments are
// handled in the manner of fmt.Println.
func Errorln(v ...interface{}) {
	getLogger().Errorln(v...)
}

// Warn prints to the logger if level is at least LevelWarn. Arguments are
// handled in the manner of fmt.Print.
func Warn(v ...interface{}) {
	getLogger().Warn(v...)
}

// Warnf prints to the logger if level is at least LevelWarn. Arguments are
// handled in the manner of fmt.Printf.
func Warnf(format string, v ...interface{}) {
	getLogger().Warnf(format, v...)
}

// Warnln prints to the logger if level is at least LevelWarn. Arguments are
// handled in the manner of fmt.Println.
func Warnln(v ...interface{}) {
	getLogger().Warnln(v...)
}

// Info prints to the logger if level is at least LevelInfo. Arguments are
// handled in the manner of fmt.Print.
func Info(v ...interface{}) {
	getLogger().Info(v...)
}

// Infof prints to the logger if level is at least LevelInfo. Arguments are
// handled in the manner of fmt.Printf.
func Infof(format string, v ...interface{}) {
	getLogger().Infof(format, v...)
}

// Infoln prints to the logger if level is at least LevelInfo. Arguments are
// handled in the manner of fmt.Println.
func Infoln(v ...interface{}) {
	getLogger().Infoln(v...)
}

// Debug prints to the logger if level is at least LevelDebug. Arguments are
// handled in the manner of fmt.Print.
func Debug(v ...interface{}) {
	getLogger().Debug(v...)
}

// Debugf prints to the logger if level is at least LevelDebug. Arguments are
// handled in the manner of fmt.Printf.
func Debugf(format string, v ...interface{}) {
	getLogger().Debugf(format, v...)
}

// Debugln prints to the logger if level is at least LevelDebug. Arguments are
// handled in the manner of fmt.Println.
func Debugln(v ...interface{}) {
	getLogger().Debugln(v...)
}

type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Method  string
}

// Custom logger for testing if other modules logged as expected.
type TestLogger struct {
	*logrus.Logger
	entries []LogEntry
}

func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: []LogEntry{},
	}
}

func (l *TestLogger) formatMessage(v ...interface{}) string {
	return fmt.Sprint(v...)
}

func (l *TestLogger) formatMessagef(format string, v ...interface{}) string {
	return fmt.Sprintf(format, v...)
}

func (l *TestLogger) formatMessageln(v ...interface{}) string {
	return fmt.Sprintln(v...)
}

func (l *TestLogger) addLogEntry(level logrus.Level, message string, method string) {
	l.entries = append(l.entries, LogEntry{time.Now(), level.String(), message, method})
}

func (l *TestLogger) Error(v ...interface{}) {
	l.addLogEntry(ErrorLevel, l.formatMessage(v...), "Error")
}

func (l *TestLogger) Errorf(format string, v ...interface{}) {
	l.addLogEntry(ErrorLevel, l.formatMessagef(format, v...), "Errorf")
}

func (l *TestLogger) Errorln(v ...interface{}) {
	l.addLogEntry(ErrorLevel, l.formatMessageln(v...), "Errorln")
}

func (l *TestLogger) Warn(v ...interface{}) {
	l.addLogEntry(WarnLevel, l.formatMessage(v...), "Warn")
}

func (l *TestLogger) Warnf(format string, v ...interface{}) {
	l.addLogEntry(WarnLevel, l.formatMessagef(format, v...), "Warnf")
}

func (l *TestLogger) Warnln(v ...interface{}) {
	l.addLogEntry(WarnLevel, l.formatMessageln(v...), "Warnln")
}

func (l *TestLogger) Info(v ...interface{}) {
	l.addLogEntry(InfoLevel, l.formatMessage(v...), "Info")
}

func (l *TestLogger) Infof(format string, v ...interface{}) {
	l.addLogEntry(InfoLevel, l.formatMessagef(format, v...), "Infof")
}

func (l *TestLogger) Infoln(v ...interface{}) {
	l.addLogEntry(InfoLevel, l.formatMessageln(v...), "Infoln")
}

func (l *TestLogger) Debug(v ...interface{}) {
	l.addLogEntry(DebugLevel, l.formatMessage(v...), "Debug")
}

func (l *TestLogger) Debugf(format string, v ...interface{}) {
	l.addLogEntry(DebugLevel, l.formatMessagef(format, v...), "Debugf")
}

func (l *TestLogger) Debugln(v ...interface{}) {
	l.addLogEntry(DebugLevel, l.formatMessageln(v...), "Debugln")
}

func (l *TestLogger) GetEntries() []LogEntry {
	return l.entries
}

func (l *TestLogger) Clear() {
	l.entries = []LogEntry{}
}

func (l *TestLogger) GetLastEntry() *LogEntry {
	if len(l.entries) == 0 {
		return nil
	}
	return &l.entries[len(l.entries)-1]
}

// Check if the last log entry is the expected one.
// Message is checked if it is contained in the log entry (not exact match).
func (l *TestLogger) IsLastEntry(level logrus.Level, message string, method string) bool {
	entry := l.GetLastEntry()
	if entry == nil {
		return false
	}
	return (entry.Level == level.String() &&
		entry.Method == method &&
		strings.Contains(entry.Message, message))
}
