#!/bin/bash

# Exit on any error
set -e

TEST_BINARY="/forklift/tests.test"
LOGS_DIR="/forklift/cmd/vsphere-xcopy-volume-populator/e2e-tests/logs"

# Create logs directory if it doesn't exist
mkdir -p "$LOGS_DIR"

# Check if the test binary exists
if [ ! -f "$TEST_BINARY" ]; then
    echo "Error: Test binary not found at $TEST_BINARY"
    exit 1
fi

# Check if the test binary is executable
if [ ! -x "$TEST_BINARY" ]; then
    echo "Error: Test binary is not executable at $TEST_BINARY"
    exit 1
fi

# Generate timestamp for log files
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
TEST_LOG="$LOGS_DIR/test_execution_${TIMESTAMP}.log"

# Run the e2e tests with output redirection
echo "Running e2e tests..."
echo "Test execution started at $(date)" | tee "$TEST_LOG"
echo "Logs will be saved to: $TEST_LOG"

# Execute tests with both stdout and stderr captured to the same log file
"$TEST_BINARY" 2>&1 | tee -a "$TEST_LOG"
TEST_EXIT_CODE=${PIPESTATUS[0]}

# Log completion status
echo "Test execution completed at $(date) with exit code: $TEST_EXIT_CODE" | tee -a "$TEST_LOG"

# Exit with the same code as the test binary
exit $TEST_EXIT_CODE
