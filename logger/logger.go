package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	std_log "log"

	go_log "git.sr.ht/~spc/go-log"
)

const defaultLogFormat = 0
const defaultLogPrefix = ""

const (
	DebugLevel = "DEBUG"
	InfoLevel  = "INFO"
	WarnLevel  = "WARN"
	ErrorLevel = "ERROR"
	TraceLevel = "TRACE"
)

type Logger interface {
	// Error prints to the logger if level is at least LevelError. Arguments are
	// handled in the manner of fmt.Print.
	Error(v ...interface{})

	// Errorf prints to the logger if level is at least LevelError. Arguments are
	// handled in the manner of fmt.Printf.
	Errorf(format string, v ...interface{})

	// Errorln prints to the logger if level is at least LevelError. Arguments are
	// handled in the manner of fmt.Println.
	Errorln(v ...interface{})

	// Warn prints to the logger if level is at least LevelWarn. Arguments are
	// handled in the manner of fmt.Print.
	Warn(v ...interface{})

	// Warnf prints to the logger if level is at least LevelWarn. Arguments are
	// handled in the manner of fmt.Printf.
	Warnf(format string, v ...interface{})

	// Warnln prints to the logger if level is at least LevelWarn. Arguments are
	// handled in the manner of fmt.Println.
	Warnln(v ...interface{})

	// Info prints to the logger if level is at least LevelInfo. Arguments are
	// handled in the manner of fmt.Print.
	Info(v ...interface{})

	// Infof prints to the logger if level is at least LevelInfo. Arguments are
	// handled in the manner of fmt.Printf.
	Infof(format string, v ...interface{})

	// Infoln prints to the logger if level is at least LevelInfo. Arguments are
	// handled in the manner of fmt.Println.
	Infoln(v ...interface{})

	// Debug prints to the logger if level is at least LevelDebug. Arguments are
	// handled in the manner of fmt.Print.
	Debug(v ...interface{})

	// Debugf prints to the logger if level is at least LevelDebug. Arguments are
	// handled in the manner of fmt.Printf.
	Debugf(format string, v ...interface{})

	// Debugln prints to the logger if level is at least LevelDebug. Arguments are
	// handled in the manner of fmt.Println.
	Debugln(v ...interface{})

	// Trace prints to the logger if level is at least LevelTrace. Arguments are
	// handled in the manner of fmt.Print.
	Trace(v ...interface{})

	// Tracef prints to the logger if level is at least LevelTrace. Arguments are
	// handled in the manner of fmt.Printf.
	Tracef(format string, v ...interface{})

	// Traceln prints to the logger if level is at least LevelTrace. Arguments are
	// handled in the manner of fmt.Println.
	Traceln(v ...interface{})
}

func InitDefaultLogger() Logger {
	return go_log.New(os.Stderr, defaultLogPrefix, defaultLogFormat, go_log.LevelDebug)
}

func InitLogger(file string, level string, prefix string, flag int) error {
	logLevel, err := go_log.ParseLevel(level)

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

	log = go_log.New(logFile, prefix, flag, logLevel)

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

// Trace prints to the logger if level is at least LevelTrace. Arguments are
// handled in the manner of fmt.Print.
func Trace(v ...interface{}) {
	getLogger().Trace(v...)
}

// Tracef prints to the logger if level is at least LevelTrace. Arguments are
// handled in the manner of fmt.Printf.
func Tracef(format string, v ...interface{}) {
	getLogger().Tracef(format, v...)
}

// Traceln prints to the logger if level is at least LevelTrace. Arguments are
// handled in the manner of fmt.Println.
func Traceln(v ...interface{}) {
	getLogger().Traceln(v...)
}

func ParseLogPrefix(format string) (prefix string, flag int) {
	if !strings.Contains(format, "%") {
		return format, defaultLogFormat
	}

	prefix = format[:strings.Index(format, "%")]
	flag = 0

	if strings.Contains(format, "%d") {
		flag |= std_log.Ldate
	}
	if strings.Contains(format, "%t") {
		flag |= std_log.Ltime
	}
	if strings.Contains(format, "%m") {
		flag |= std_log.Lmicroseconds
	}
	if strings.Contains(format, "%l") {
		flag |= std_log.Llongfile
	}
	if strings.Contains(format, "%s") {
		flag |= std_log.Lshortfile
	}
	if strings.Contains(format, "%z") {
		flag |= std_log.LUTC
	}
	if strings.Contains(format, "%p") {
		flag |= std_log.Lmsgprefix
	}
	if strings.Contains(format, "%S") {
		flag |= std_log.LstdFlags
	}

	return prefix, flag

}

type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Method  string
}

// Custom logger for testing if other modules logged as expected.
type TestLogger struct {
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

func (l *TestLogger) addLogEntry(level string, message string, method string) {
	l.entries = append(l.entries, LogEntry{time.Now(), level, message, method})
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

func (l *TestLogger) Trace(v ...interface{}) {
	l.addLogEntry(TraceLevel, l.formatMessage(v...), "Trace")
}

func (l *TestLogger) Tracef(format string, v ...interface{}) {
	l.addLogEntry(TraceLevel, l.formatMessagef(format, v...), "Tracef")
}

func (l *TestLogger) Traceln(v ...interface{}) {
	l.addLogEntry(TraceLevel, l.formatMessageln(v...), "Traceln")
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
func (l *TestLogger) IsLastEntry(level string, message string, method string) bool {
	entry := l.GetLastEntry()
	if entry == nil {
		return false
	}
	return (entry.Level == level &&
		entry.Method == method &&
		strings.Contains(entry.Message, message))
}
