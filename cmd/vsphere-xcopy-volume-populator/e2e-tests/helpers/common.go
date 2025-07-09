package helpers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Colors for output
const (
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[1;33m"
	ColorBlue   = "\033[0;34m"
	ColorNC     = "\033[0m" // No Color
)

// VM Disk Types
const (
	DiskThin           = "thin"
	DiskThick          = "thick"
	DiskEagerZeroedThick = "eagerzeroedthick"
)

// Logger handles logging functionality
type Logger struct {
	logFile   *os.File
	startTime time.Time
	debugMode bool
	mu        sync.Mutex
}

// NewLogger creates a new logger instance
func NewLogger(logDir, testName string, debugMode bool) (*Logger, error) {
	if logDir == "" {
		// Default to a 'logs' directory inside the tests directory.
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %v", err)
		}
		logDir = filepath.Join(wd, "logs")
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	if testName == "" {
		testName = os.Getenv("TEST_NAME")
	}
	if testName == "" {
		testName = "e2e-test"
	}
	// Sanitize the test name to be filesystem-friendly
	testName = strings.ReplaceAll(testName, "/", "_")

	timestamp := time.Now().Format("20060102_150405")
	randSuffix, err := GenerateRandomString(4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random suffix for log file: %w", err)
	}

	logFileName := filepath.Join(logDir, fmt.Sprintf("%s_%s_%s.log", testName, timestamp, randSuffix))

	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logFileName, err)
	}

	logger := &Logger{
		logFile:   file,
		startTime: time.Now(),
		debugMode: debugMode,
	}

	logger.LogInfo("Starting e2e test at %s", time.Now().Format("2006-01-02 15:04:05"))
	logger.LogInfo("Log file: %s", logFileName)

	return logger, nil
}

// Close safely closes the log file.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logFile != nil {
		if err := l.logFile.Close(); err != nil {
			// Log the closing error to stderr, as the file logger might be unavailable.
			fmt.Fprintf(os.Stderr, "Error closing log file: %v\n", err)
		}
		l.logFile = nil // Prevent double close
	}
}

// log writes a log entry with level and message
func (l *Logger) log(level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s: %s", timestamp, level, message)

	// Get color based on level
	var color string
	switch level {
	case "ERROR":
		color = ColorRed
	case "WARN":
		color = ColorYellow
	case "INFO":
		color = ColorGreen
	case "DEBUG":
		color = ColorBlue
	default:
		color = ColorNC
	}

	// Print to console with color
	fmt.Printf("%s%s%s\n", color, logEntry, ColorNC)

	// Write to log file without color
	if l.logFile != nil {
		if _, err := fmt.Fprintf(l.logFile, "%s\n", logEntry); err != nil {
			// If logging to file fails, print an error to stderr.
			fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		}
	}
}

// LogInfo logs an info message
func (l *Logger) LogInfo(format string, args ...interface{}) {
	l.log("INFO", fmt.Sprintf(format, args...))
}

// LogWarn logs a warning message
func (l *Logger) LogWarn(format string, args ...interface{}) {
	l.log("WARN", fmt.Sprintf(format, args...))
}

// LogError logs an error message
func (l *Logger) LogError(format string, args ...interface{}) {
	l.log("ERROR", fmt.Sprintf(format, args...))
}

// LogDebug logs a debug message if debug mode is enabled
func (l *Logger) LogDebug(format string, args ...interface{}) {
	if l.debugMode {
		l.log("DEBUG", fmt.Sprintf(format, args...))
	}
}

// CheckRequiredVars checks if required environment variables are set
func CheckRequiredVars(vars ...string) error {
	var missingVars []string

	for _, v := range vars {
		if os.Getenv(v) == "" {
			missingVars = append(missingVars, v)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	return nil
}

// CheckRequiredTools checks if required tools are available
func CheckRequiredTools(tools ...string) error {
	var missingTools []string

	for _, tool := range tools {
		if !isCommandAvailable(tool) {
			missingTools = append(missingTools, tool)
		}
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missingTools, ", "))
	}

	return nil
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(ctx context.Context, conditionFunc func() bool, timeout time.Duration, interval time.Duration, description string, logger *Logger) error {
	logger.LogInfo("Waiting for %s (timeout: %v)", description, timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s after %v", description, time.Since(start))
		case <-ticker.C:
			if conditionFunc() {
				elapsed := time.Since(start)
				logger.LogInfo("%s satisfied after %v", description, elapsed)
				return nil
			}
			elapsed := time.Since(start)
			logger.LogDebug("Waiting for %s... (%v/%v)", description, elapsed, timeout)
		}
	}
}

// GenerateRandomString generates a cryptographically secure random string of a specified length.
func GenerateRandomString(length int) (string, error) {
	if length < 0 {
		return "", fmt.Errorf("length must be non-negative")
	}

	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		result[i] = chars[num.Int64()]
	}
	return string(result), nil
}
