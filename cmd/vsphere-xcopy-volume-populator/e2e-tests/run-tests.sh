#!/bin/bash

# Security-focused shell options
set -euo pipefail  # Exit on error, undefined vars, pipe failures
IFS=$'\n\t'        # Secure IFS to prevent word splitting issues

# Default paths - can be overridden by environment variables
# Input validation: ensure paths don't contain dangerous characters
validate_path() {
    local path="$1"
    local name="$2"

    # Debug: show what we're validating
    echo "Debug: Validating $name = '$path'" >&2
    echo "Debug: Path length = ${#path}" >&2

    # Check for empty path
    if [[ -z "$path" ]]; then
        echo "Error: $name is empty" >&2
        exit 1
    fi

    # Check for obviously dangerous characters in a simpler way
    # Only check for newlines and obvious path traversal - skip null byte check as it's problematic
    case "$path" in
        *$'\n'*)
            echo "Error: $name contains newline characters" >&2
            exit 1
            ;;
        *$'\r'*)
            echo "Error: $name contains carriage return characters" >&2
            exit 1
            ;;
        */../*|*/..|../*|..)
            echo "Error: $name contains path traversal sequences" >&2
            exit 1
            ;;
    esac

    # Additional check for very suspicious patterns
    if [[ "$path" == *';'* ]] || [[ "$path" == *'|'* ]] || [[ "$path" == *'&'* ]]; then
        echo "Error: $name contains shell metacharacters (;|&)" >&2
        exit 1
    fi

    echo "Debug: $name validation passed" >&2
}

TEST_BINARY="${TEST_BINARY_PATH:-/forklift/tests.test}"
LOGS_DIR="${E2E_LOG_DIR:-/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs}"

# Validate inputs (can be disabled by setting SKIP_PATH_VALIDATION=true)
if [[ "${SKIP_PATH_VALIDATION:-false}" != "true" ]]; then
    validate_path "$TEST_BINARY" "TEST_BINARY"
    validate_path "$LOGS_DIR" "LOGS_DIR"
else
    echo "Warning: Path validation skipped due to SKIP_PATH_VALIDATION=true" >&2
fi

# Create logs directory if it doesn't exist (with proper error handling)
if ! mkdir -p "$LOGS_DIR"; then
    echo "Error: Failed to create logs directory: $LOGS_DIR" >&2
    exit 1
fi

# Check if the test binary exists
if [[ ! -f "$TEST_BINARY" ]]; then
    echo "Error: Test binary not found at: $TEST_BINARY" >&2
    exit 1
fi

# Check if the test binary is executable
if [[ ! -x "$TEST_BINARY" ]]; then
    echo "Error: Test binary is not executable at: $TEST_BINARY" >&2
    exit 1
fi

# Generate timestamp for log files (using safer date format)
TIMESTAMP=$(date '+%Y%m%d_%H%M%S' 2>/dev/null) || {
    echo "Error: Failed to generate timestamp" >&2
    exit 1
}
TEST_LOG="$LOGS_DIR/test_execution_${TIMESTAMP}.log"

# Validate the generated log file path
validate_path "$TEST_LOG" "TEST_LOG"

# Run the e2e tests with output redirection
echo "Running e2e tests..."

# Initialize log file securely
if ! echo "Test execution started at $(date)" | tee "$TEST_LOG" >/dev/null; then
    echo "Error: Failed to write to log file: $TEST_LOG" >&2
    exit 1
fi

echo "Logs will be saved to: $TEST_LOG"

# Execute tests with both stdout and stderr captured to the same log file
# Use explicit exec to ensure proper signal handling
if ! "$TEST_BINARY" 2>&1 | tee -a "$TEST_LOG"; then
    TEST_EXIT_CODE=${PIPESTATUS[0]:-1}
else
    TEST_EXIT_CODE=${PIPESTATUS[0]:-0}
fi

# Ensure we have a valid exit code
if [[ ! "$TEST_EXIT_CODE" =~ ^[0-9]+$ ]]; then
    echo "Warning: Invalid exit code detected, defaulting to 1" >&2
    TEST_EXIT_CODE=1
fi

# Log completion status with error handling
{
    echo "Test execution completed at $(date) with exit code: $TEST_EXIT_CODE"
} | tee -a "$TEST_LOG" || {
    echo "Warning: Failed to write completion status to log file" >&2
}

# Exit with the same code as the test binary
exit "$TEST_EXIT_CODE"
