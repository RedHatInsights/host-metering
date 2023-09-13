package logger

import (
	"os"

	go_log "git.sr.ht/~spc/go-log"
)

const defaultLogFormat = 0

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
	return go_log.New(os.Stderr, "", defaultLogFormat, go_log.LevelDebug)
}

func InitLogger(file string, level string, logStructure ...int) error {
	logLevel, err := go_log.ParseLevel(level)

	if err != nil {
		return err
	}

	actualLogStructure := defaultLogFormat
	if len(logStructure) > 0 {
		actualLogStructure = logStructure[0]
	}

	logFile := os.Stderr
	if file != "" {
		logFile, err = os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	}
	if err != nil {
		return err
	}

	log = go_log.New(logFile, "", actualLogStructure, logLevel)

	return nil
}

// Inject a predefined logger instance, will be used for testing.
func OverrideLogger(newInstance *Logger) {
	log = *newInstance
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
