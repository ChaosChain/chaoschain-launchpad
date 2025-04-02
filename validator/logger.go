package validator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// LogCategory defines the different categories of log events
type LogCategory string

const (
	MEMORY     LogCategory = "MEMORY"
	SOCIAL     LogCategory = "SOCIAL"
	LEARNING   LogCategory = "LEARNING"
	VALIDATION LogCategory = "VALIDATION"
	DISCUSSION LogCategory = "DISCUSSION"
	ERROR      LogCategory = "ERROR"
	TASK       LogCategory = "TASK"
	BLOCK      LogCategory = "BLOCK"
	SYSTEM     LogCategory = "SYSTEM"
)

// Logger provides structured logging for validators with different log categories
type Logger struct {
	ValidatorID   string
	ValidatorName string
	ChainID       string
	LogFile       *os.File
	ConsoleLogger *log.Logger
	FileLogger    *log.Logger
}

// NewLogger creates a new logger for a validator
func NewLogger(validatorID, validatorName, chainID string) *Logger {
	// Set up console logger
	consoleLogger := log.New(os.Stdout, "", log.LstdFlags)

	// Attempt to set up file logger
	var fileLogger *log.Logger
	var logFile *os.File

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err == nil {
		// Create chainID subdirectory if needed
		chainDir := "logs"
		if chainID != "" {
			chainDir = filepath.Join("logs", chainID)
			if err := os.MkdirAll(chainDir, 0755); err != nil {
				log.Printf("Warning: Could not create chain log directory: %v", err)
			}
		}

		// Create a log file for this validator
		logFileName := fmt.Sprintf("%s_%s.log", validatorID, time.Now().Format("20060102_150405"))
		logFilePath := filepath.Join(chainDir, logFileName)

		// Try to open the log file
		if f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			logFile = f
			fileLogger = log.New(f, "", log.LstdFlags)
		} else {
			log.Printf("Warning: Could not create log file: %v", err)
		}
	}

	return &Logger{
		ValidatorID:   validatorID,
		ValidatorName: validatorName,
		ChainID:       chainID,
		LogFile:       logFile,
		ConsoleLogger: consoleLogger,
		FileLogger:    fileLogger,
	}
}

// Close closes the log file if it's open
func (l *Logger) Close() {
	if l.LogFile != nil {
		l.LogFile.Close()
	}
}

// formatLogEntry creates a consistently formatted log entry
func (l *Logger) formatLogEntry(category LogCategory, action, target string, format string, args ...interface{}) string {
	message := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] [%s] [%s:%s] %s",
		l.ValidatorName,
		string(category),
		action,
		target,
		message)
}

// logEntry logs an entry to both console and file
func (l *Logger) logEntry(category LogCategory, action, target string, format string, args ...interface{}) {
	entry := l.formatLogEntry(category, action, target, format, args...)

	// Log to console
	l.ConsoleLogger.Println(entry)

	// Log to file if available
	if l.FileLogger != nil {
		l.FileLogger.Println(entry)
	}
}

// Info logs general information
func (l *Logger) Info(category LogCategory, format string, args ...interface{}) {
	l.logEntry(category, "INFO", "", format, args...)
}

// Memory logs memory-related operations
func (l *Logger) Memory(action, format string, args ...interface{}) {
	l.logEntry(MEMORY, action, "", format, args...)
}

// Social logs social interactions and mood changes
func (l *Logger) Social(action, target string, format string, args ...interface{}) {
	l.logEntry(SOCIAL, action, target, format, args...)
}

// Learning logs reinforcement learning events
func (l *Logger) Learning(action string, format string, args ...interface{}) {
	l.logEntry(LEARNING, action, "", format, args...)
}

// Validation logs block validation events
func (l *Logger) Validation(blockHeight int, blockHash string, format string, args ...interface{}) {
	l.logEntry(VALIDATION, "Validate", fmt.Sprintf("Block:%d:%s", blockHeight, blockHash), format, args...)
}

// Discussion logs block discussion messages
func (l *Logger) Discussion(blockHash string, format string, args ...interface{}) {
	l.logEntry(DISCUSSION, "Discuss", blockHash, format, args...)
}

// Task logs task-related actions
func (l *Logger) Task(action, taskID string, format string, args ...interface{}) {
	l.logEntry(TASK, action, taskID, format, args...)
}

// Error logs error conditions
func (l *Logger) Error(context string, format string, args ...interface{}) {
	l.logEntry(ERROR, context, "", format, args...)
}

// System logs system-level messages
func (l *Logger) System(action string, format string, args ...interface{}) {
	l.logEntry(SYSTEM, action, "", format, args...)
}
