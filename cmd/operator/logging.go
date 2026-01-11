package main

import (
	"log"
	"os"
	"strings"
)

// LogLevel represents logging severity
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
)

// InitializeLogging sets up structured logging with proper levels
func InitializeLogging(debugEnabled bool) {
	log.SetOutput(os.Stderr)

	// Add timestamp and file location to all log messages
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// StructuredLogger provides categorized logging
type StructuredLogger struct {
	level LogLevel
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(level LogLevel) *StructuredLogger {
	return &StructuredLogger{level: level}
}

// Error logs an error message
func (l *StructuredLogger) Error(msg string, keysValues ...interface{}) {
	if l.level >= LogLevelError {
		log.Printf("ERROR: %s %+v", msg, keysValues)
	}
}

// Warn logs a warning message
func (l *StructuredLogger) Warn(msg string, keysValues ...interface{}) {
	if l.level >= LogLevelWarn {
		log.Printf("WARN: %s %+v", msg, keysValues)
	}
}

// Info logs an info message
func (l *StructuredLogger) Info(msg string, keysValues ...interface{}) {
	if l.level >= LogLevelInfo {
		log.Printf("INFO: %s %+v", msg, keysValues)
	}
}

// Debug logs a debug message
func (l *StructuredLogger) Debug(msg string, keysValues ...interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("DEBUG: %s %+v", msg, keysValues)
	}
}

// Trace logs a trace message
func (l *StructuredLogger) Trace(msg string, keysValues ...interface{}) {
	if l.level >= LogLevelTrace {
		log.Printf("TRACE: %s %+v", msg, keysValues)
	}
}

// GetLogLevelFromEnv returns log level from environment variable
func GetLogLevelFromEnv() LogLevel {
	level := os.Getenv("LOG_LEVEL")
	level = strings.ToUpper(level)

	switch level {
	case "ERROR":
		return LogLevelError
	case "WARN":
		return LogLevelWarn
	case "INFO":
		return LogLevelInfo
	case "DEBUG":
		return LogLevelDebug
	case "TRACE":
		return LogLevelTrace
	default:
		return LogLevelInfo // Default to INFO
	}
}

// FormatLogMessage formats a log message with correlation ID
func FormatLogMessage(correlationID, namespace, crName, phase string, details map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(correlationID)
	sb.WriteString("] ")
	sb.WriteString(namespace)
	sb.WriteString("/")
	sb.WriteString(crName)
	sb.WriteString(" phase=")
	sb.WriteString(phase)

	for k, v := range details {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(formatValue(v))
	}

	return sb.String()
}

// formatValue formats a value for logging
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return string(rune(val))
	case int32:
		return string(rune(val))
	case int64:
		return string(rune(val))
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "nil"
	default:
		return ""
	}
}
