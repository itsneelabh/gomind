package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// SimpleLogger provides a basic structured logger implementation
type SimpleLogger struct {
	level  LogLevel
	fields map[string]interface{}
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{
		level:  InfoLevel,
		fields: make(map[string]interface{}),
	}
}

// NewDefaultLogger creates a new default logger instance
func NewDefaultLogger() Logger {
	return NewSimpleLogger()
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, fields ...interface{}) {
	if l.level <= DebugLevel {
		l.log("DEBUG", msg, fields...)
	}
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, fields ...interface{}) {
	if l.level <= InfoLevel {
		l.log("INFO", msg, fields...)
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, fields ...interface{}) {
	if l.level <= WarnLevel {
		l.log("WARN", msg, fields...)
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, fields ...interface{}) {
	if l.level <= ErrorLevel {
		l.log("ERROR", msg, fields...)
	}
}

// SetLevel sets the logging level
func (l *SimpleLogger) SetLevel(level string) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		l.level = DebugLevel
	case "INFO":
		l.level = InfoLevel
	case "WARN", "WARNING":
		l.level = WarnLevel
	case "ERROR":
		l.level = ErrorLevel
	}
}

// WithField returns a logger with an additional field
func (l *SimpleLogger) WithField(key string, value interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &SimpleLogger{
		level:  l.level,
		fields: newFields,
	}
}

// WithFields returns a logger with additional fields
func (l *SimpleLogger) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &SimpleLogger{
		level:  l.level,
		fields: newFields,
	}
}

// With returns a logger with additional fields
func (l *SimpleLogger) With(fields ...Field) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for _, f := range fields {
		newFields[f.Key] = f.Value
	}

	return &SimpleLogger{
		level:  l.level,
		fields: newFields,
	}
}

// log performs the actual logging
func (l *SimpleLogger) log(level, msg string, fields ...interface{}) {
	// Build the log message
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", level))
	parts = append(parts, msg)

	// Add structured fields
	for k, v := range l.fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	// Add additional fields from arguments
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			parts = append(parts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
		}
	}

	log.Println(strings.Join(parts, " "))
}

// GetLogLevel gets the current log level from environment
func GetLogLevel() string {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		return "INFO"
	}
	return level
}
