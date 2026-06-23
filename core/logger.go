package core

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelSilent
)

var levelNames = map[LogLevel]string{
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
}

type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  LogLevel
	prefix string
}

var (
	defaultLogger = &Logger{
		out:    os.Stderr,
		level:  LogLevelInfo,
		prefix: "CroMedia",
	}
	logMu sync.RWMutex
)

// SetLogLevel sets the global log level.
func SetLogLevel(lvl LogLevel) {
	logMu.Lock()
	defer logMu.Unlock()
	defaultLogger.level = lvl
}

// GetLogLevel gets the current global log level.
func GetLogLevel() LogLevel {
	logMu.RLock()
	defer logMu.RUnlock()
	return defaultLogger.level
}

// Log logs a structured message.
func Log(lvl LogLevel, msg string, args ...interface{}) {
	logMu.RLock()
	defer logMu.RUnlock()

	if lvl < defaultLogger.level {
		return
	}

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	lvlName := levelNames[lvl]
	formatted := fmt.Sprintf(msg, args...)

	fmt.Fprintf(defaultLogger.out, "[%s] [%s] [%s] %s\n", timestamp, defaultLogger.prefix, lvlName, formatted)
}

// Debug logs a debug message.
func Debug(msg string, args ...interface{}) {
	Log(LogLevelDebug, msg, args...)
}

// Info logs an info message.
func Info(msg string, args ...interface{}) {
	Log(LogLevelInfo, msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...interface{}) {
	Log(LogLevelWarn, msg, args...)
}

// Error logs an error message.
func Error(msg string, args ...interface{}) {
	Log(LogLevelError, msg, args...)
}
