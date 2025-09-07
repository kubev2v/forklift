// Package helpers provides common utilities and helpers for the vSphere XCOPY volume populator e2e tests.
// This package includes logging functionality, validation utilities, secure command execution,
// and other common operations needed across the testing framework.
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
	DiskThin             = "thin"
	DiskThick            = "thick"
	DiskEagerZeroedThick = "eagerzeroedthick"
)

// Default paths and configuration constants
const (
	DefaultLogDir          = "/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs"
	DefaultTestBinaryPath  = "/forklift/tests.test"
	DefaultFilePermissions = 0644
	DefaultDirPermissions  = 0755
	LogTimestampFormat     = "20060102_150405"
	LogDateTimeFormat      = "2006-01-02 15:04:05"
)

// Environment variable names
const (
	EnvLogDir   = "E2E_LOG_DIR"
	EnvTestName = "TEST_NAME"
)

// Timeout and interval constants
const (
	DefaultPlanTimeoutSeconds      = 500 // 500 seconds for plan readiness
	DefaultMigrationTimeoutSeconds = 500 // 500 seconds for migration completion
	DefaultVMBootTimeoutSeconds    = 300 // 300 seconds for VM boot
	DefaultPollingIntervalSeconds  = 15  // 15 seconds between status checks
	DefaultRetryIntervalSeconds    = 30  // 30 seconds between retries
)

// Network and port constants
const (
	DefaultOpenShiftAPIPort = 6443
	DefaultSSHPort          = 22
)

// VM default configuration constants
const (
	DefaultVMDiskSizeGB        = 20
	DefaultVMMemoryMB          = 2048
	DefaultVMCPUCount          = 2
	DefaultMigrationTimeoutMin = 60
	RandomSuffixLength         = 4
	VMNameRandomSuffixLength   = 6
)

// String length constants
const (
	DNS1123MaxLength = 63 // Maximum length for DNS-1123 labels
)

// Logger handles logging functionality for the e2e test framework.
// It provides thread-safe logging to both console (with colors) and log files,
// with support for different log levels and debug mode.
type Logger struct {
	logFile   *os.File   // File handle for log output
	startTime time.Time  // Start time for duration calculations
	debugMode bool       // Whether debug logging is enabled
	mu        sync.Mutex // Mutex for thread-safe operations
}

// NewLogger creates a new logger instance with input validation
func NewLogger(logDir, testName string, debugMode bool) (*Logger, error) {
	if logDir == "" {
		// Default to a 'logs' directory inside the tests directory.
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		logDir = filepath.Join(wd, "logs")
	}

	// Validate logDir is a valid path
	if !filepath.IsAbs(logDir) && !strings.HasPrefix(logDir, ".") && !strings.HasPrefix(logDir, "/") {
		return nil, fmt.Errorf("invalid log directory path: %s", logDir)
	}

	if err := os.MkdirAll(logDir, DefaultDirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	if testName == "" {
		testName = os.Getenv(EnvTestName)
	}
	if testName == "" {
		testName = "e2e-test"
	}
	// Sanitize the test name to be filesystem-friendly
	testName = strings.ReplaceAll(testName, "/", "_")

	timestamp := time.Now().Format(LogTimestampFormat)
	randSuffix, err := GenerateRandomString(RandomSuffixLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random suffix for log file: %w", err)
	}

	logFileName := filepath.Join(logDir, fmt.Sprintf("%s_%s_%s.log", testName, timestamp, randSuffix))

	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, DefaultFilePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logFileName, err)
	}

	logger := &Logger{
		logFile:   file,
		startTime: time.Now(),
		debugMode: debugMode,
	}

	logger.LogInfo("Starting e2e test at %s", time.Now().Format(LogDateTimeFormat))
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
	if l == nil {
		// Fallback to stderr if logger is nil
		fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", time.Now().Format(LogDateTimeFormat), level, message)
		return
	}

	if level == "" {
		level = "INFO"
	}

	if message == "" {
		message = "(empty message)"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(LogDateTimeFormat)
	logEntry := fmt.Sprintf("[%s] %s: %s", timestamp, level, message)

	// Check if NO_COLOR environment variable is set
	noColor := os.Getenv("NO_COLOR") != ""

	// Print to console with or without color based on NO_COLOR
	if noColor {
		// Print without color when NO_COLOR is set
		fmt.Printf("%s\n", logEntry)
	} else {
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
	}

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
	missingVars := findMissingVars(vars...)
	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}
	return nil
}

// findMissingVars returns a list of environment variables that are not set
func findMissingVars(vars ...string) []string {
	var missingVars []string
	for _, v := range vars {
		if os.Getenv(v) == "" {
			missingVars = append(missingVars, v)
		}
	}
	return missingVars
}

// CheckRequiredTools checks if required tools are available
func CheckRequiredTools(tools ...string) error {
	missingTools := findMissingTools(tools...)
	if len(missingTools) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missingTools, ", "))
	}
	return nil
}

// findMissingTools returns a list of tools that are not available in PATH
func findMissingTools(tools ...string) []string {
	var missingTools []string
	for _, tool := range tools {
		if !isCommandAvailable(tool) {
			missingTools = append(missingTools, tool)
		}
	}
	return missingTools
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(ctx context.Context, conditionFunc func() bool, timeout time.Duration, interval time.Duration, description string, logger *Logger) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if conditionFunc == nil {
		return fmt.Errorf("condition function cannot be nil")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", timeout)
	}
	if interval <= 0 {
		return fmt.Errorf("interval must be positive, got: %v", interval)
	}
	if description == "" {
		description = "unknown condition"
	}
	if logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}

	logger.LogInfo("Waiting for %s (timeout: %v)", description, timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop() // Ensure ticker is always stopped to prevent resource leak

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			// Log the final state before timing out
			logger.LogWarn("Timeout waiting for %s after %v (context error: %v)", description, elapsed, ctx.Err())
			return fmt.Errorf("timeout waiting for %s after %v: %w", description, elapsed, ctx.Err())
		case <-ticker.C:
			// Check if the condition is met
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

// SecureExecCommand creates an exec.Cmd with a secure PATH containing only fixed, unwriteable directories.
// This prevents PATH injection attacks by ensuring only system directories are used.
func SecureExecCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)

	// Set a secure PATH with only fixed, unwriteable system directories
	// These are standard system directories that should be read-only for non-root users
	securePath := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// Get current environment and replace/set PATH
	env := os.Environ()
	var newEnv []string
	pathSet := false

	for _, envVar := range env {
		if strings.HasPrefix(envVar, "PATH=") {
			// Replace existing PATH with secure version
			newEnv = append(newEnv, "PATH="+securePath)
			pathSet = true
		} else {
			newEnv = append(newEnv, envVar)
		}
	}

	// If PATH wasn't in the environment, add it
	if !pathSet {
		newEnv = append(newEnv, "PATH="+securePath)
	}

	cmd.Env = newEnv
	return cmd
}

// FormatEnvVar creates a properly formatted environment variable string
func FormatEnvVar(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}

// FormatEnvVars converts a map of key-value pairs to environment variable strings
func FormatEnvVars(vars map[string]string) []string {
	result := make([]string, 0, len(vars))
	for key, value := range vars {
		result = append(result, FormatEnvVar(key, value))
	}
	return result
}
