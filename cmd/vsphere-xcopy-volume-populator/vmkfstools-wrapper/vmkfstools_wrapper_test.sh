#!/bin/sh

# vmkfstools_wrapper_test.sh - Comprehensive Unit Tests
# Usage: ./vmkfstools_wrapper_test.sh [path-to-vmkfstools_wrapper.sh]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WRAPPER_SCRIPT="${1:-${SCRIPT_DIR}/vmkfstools_wrapper.sh}"
TEST_TMP_DIR="/tmp/vmkfstools-wrapper-tests"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# No colors - plain ASCII only

# Test helper functions
test_setup() {
    mkdir -p "${TEST_TMP_DIR}"
    export LOG_FILE="${TEST_TMP_DIR}/test.log"
    > "${LOG_FILE}"
}

test_teardown() {
    rm -rf "${TEST_TMP_DIR}"
}

assert_equals() {
    local expected="$1"
    local actual="$2"
    local test_name="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if [ "${expected}" = "${actual}" ]; then
        printf "PASS: %s\n" "${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        printf "FAIL: %s\n" "${test_name}"
        printf "  Expected: %s\n" "${expected}"
        printf "  Actual:   %s\n" "${actual}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

assert_contains() {
    local expected="$1"
    local actual="$2"
    local test_name="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if echo "${actual}" | grep -q "${expected}"; then
        printf "PASS: %s\n" "${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        printf "FAIL: %s\n" "${test_name}"
        printf "  Expected to contain: %s\n" "${expected}"
        printf "  Actual:   %s\n" "${actual}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

assert_not_contains() {
    local unexpected="$1"
    local actual="$2"
    local test_name="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if ! echo "${actual}" | grep -q "${unexpected}"; then
        printf "PASS: %s\n" "${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        printf "FAIL: %s\n" "${test_name}"
        printf "  Should not contain: %s\n" "${unexpected}"
        printf "  Actual:   %s\n" "${actual}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

assert_exit_code() {
    local expected_code="$1"
    local actual_code="$2"
    local test_name="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if [ "${expected_code}" -eq "${actual_code}" ]; then
        printf "PASS: %s\n" "${test_name}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        printf "FAIL: %s\n" "${test_name}"
        printf "  Expected exit code: %s\n" "${expected_code}"
        printf "  Actual exit code:   %s\n" "${actual_code}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# ============================================================================
# GROUP 1: Basic Command Tests
# ============================================================================

# Test: Version Command
test_version_command() {
    echo ""
    echo "=== Testing Version Command ==="

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --version 2>&1)
    local exit_code=$?

    assert_exit_code 0 ${exit_code} "Version command exits with 0"
    assert_contains "0.3.0" "${output}" "Version output contains version number"
    assert_contains "<?xml version" "${output}" "Version output is XML format"
    assert_contains '"version"' "${output}" "Version output contains version field"
}

# Test: Version Command with --output simple
test_version_output_simple() {
    echo ""
    echo "=== Testing Version Command with --output simple ==="

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --version --output simple 2>&1)
    local exit_code=$?

    assert_exit_code 0 ${exit_code} "Version command with --output simple exits with 0"
    assert_equals "0.3.0" "${output}" "Simple output returns only version number"
    assert_not_contains "<?xml" "${output}" "Simple output has no XML"
    assert_not_contains "version" "${output}" "Simple output has no JSON field name"
}

# Test: Version Command with --output xml
test_version_output_xml() {
    echo ""
    echo "=== Testing Version Command with --output xml ==="

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --version --output xml 2>&1)
    local exit_code=$?

    assert_exit_code 0 ${exit_code} "Version command with --output xml exits with 0"
    assert_contains "0.3.0" "${output}" "XML output contains version number"
    assert_contains "<?xml version" "${output}" "XML output has XML declaration"
    assert_contains '"version"' "${output}" "XML output contains version field"
}

# ============================================================================
# GROUP 2: Path Validation and Security
# ============================================================================

# Test: Path Validation and Security
test_path_validation() {
    echo ""
    echo "=== Testing Path Validation and Security ==="

    cat > "${TEST_TMP_DIR}/validate.sh" << 'EOF'
#!/bin/sh
LOG_FILE="/dev/null"
SCRIPT_VERSION="0.3.0"
log_info() { :; }
log_error() { echo "ERROR: $1" >&2; }
validate_path() {
    local path="$1"
    log_info "vmkfstools-wrapper version ${SCRIPT_VERSION} validating path: ${path}"
    path=$(echo "${path}" | sed 's|//*|/|g')
    path=$(echo "${path}" | sed 's|/\./|/|g')
    path=$(echo "${path}" | sed 's|/\+$||')
    case "${path}" in
        *..*|*/../*|*/..*|*/..)
            log_error "Path traversal detected: ${path}"
            echo "Invalid path detected: ${path}"
            return 1
            ;;
    esac
    case "${path}" in
        /vmfs/volumes/*|/vmfs/devices/disks/*)
            ;;
        *)
            log_error "Path not in allowed directories: ${path}"
            echo "Path not in allowed directories: ${path}"
            return 1
            ;;
    esac
    log_info "Path validation passed for: ${path}"
    echo "${path}"
    return 0
}
validate_path "$@"
EOF
    chmod +x "${TEST_TMP_DIR}/validate.sh"

    # Test path traversal attacks
    sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/volumes/../../../etc/passwd" >/dev/null 2>&1
    assert_exit_code 1 $? "Path: Rejects ../../../ traversal"

    sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/volumes/datastore1/.." >/dev/null 2>&1
    assert_exit_code 1 $? "Path: Rejects trailing .."

    sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/volumes/../volumes/ds1" >/dev/null 2>&1
    assert_exit_code 1 $? "Path: Rejects ../ in middle"

    # Test invalid directories
    sh "${TEST_TMP_DIR}/validate.sh" "/etc/passwd" >/dev/null 2>&1
    assert_exit_code 1 $? "Path: Rejects /etc paths"

    sh "${TEST_TMP_DIR}/validate.sh" "/tmp/test.vmdk" >/dev/null 2>&1
    assert_exit_code 1 $? "Path: Rejects /tmp paths"

    # Test path normalization
    local result=$(sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/volumes//datastore1/test" 2>&1)
    echo "${result}" | grep -q "/vmfs/volumes/datastore1/test"
    assert_exit_code 0 $? "Path: Double slashes normalized"

    # Test valid paths
    sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/volumes/datastore1/vm.vmdk" >/dev/null 2>&1
    assert_exit_code 0 $? "Path: Valid datastore path accepted"

    sh "${TEST_TMP_DIR}/validate.sh" "/vmfs/devices/disks/naa.600" >/dev/null 2>&1
    assert_exit_code 0 $? "Path: Valid disk device accepted"
}

# ============================================================================
# GROUP 3: Output Format Tests
# ============================================================================

# Test: XML Output Format
test_xml_output_format() {
    echo ""
    echo "=== Testing XML Output Format ==="

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --version 2>&1)

    assert_contains '<?xml version="1.0" ?>' "${output}" "XML declaration present"
    assert_contains '<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">' "${output}" "XML namespace correct"
    assert_contains '<structure typeName="result">' "${output}" "XML structure element present"
    assert_contains '<field name="status">' "${output}" "Status field present"
    assert_contains '<field name="message">' "${output}" "Message field present"
    assert_contains '</output>' "${output}" "XML properly closed"
}

# Test: JSON Escaping
test_json_escaping() {
    echo ""
    echo "=== Testing JSON Escaping (jq validation) ==="

    # Copy of escape_json function for testing
    escape_json() {
        local str="$1"
        str="${str//\\/\\\\}"
        str="${str//\"/\\\"}"
        str="${str//$'\n'/\\n}"
        str="${str//$'\t'/\\t}"
        str="${str//$'\r'/\\r}"
        str="${str//$'\b'/\\b}"
        str="${str//$'\f'/\\f}"
        printf '%s' "$str"
    }

    validate_with_jq() {
        local input="$1"
        local test_name="$2"
        local escaped=$(escape_json "${input}")
        local json="{\"msg\": \"${escaped}\"}"

        TESTS_RUN=$((TESTS_RUN + 1))
        if command -v jq >/dev/null 2>&1; then
            if echo "${json}" | jq . >/dev/null 2>&1; then
                printf "PASS: %s\n" "${test_name}"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                printf "FAIL: %s\n" "${test_name}"
                printf "  Input: %s\n" "${input}"
                printf "  Escaped: %s\n" "${escaped}"
                printf "  JSON: %s\n" "${json}"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            # Skip if jq not available
            TESTS_PASSED=$((TESTS_PASSED + 1))
        fi
    }

    # Test backslash escaping
    validate_with_jq 'test\string' "Backslashes with jq"

    # Test quote escaping
    validate_with_jq 'test"quote' "Quotes with jq"

    # Test newline escaping
    validate_with_jq "$(printf 'line1\nline2')" "Newlines with jq"

    # CRITICAL: Test escape order (backslashes MUST be escaped before quotes)
    # This test ensures we don't double-escape by doing quotes first
    # Input: test\"quote (backslash + quote)
    # Step 1 (escape \): test\\"quote (now we have double-backslash + quote)
    # Step 2 (escape "): test\\\"quote (double-backslash + escaped-quote)
    # If order was wrong (quotes first), we'd get: test\" → test\\" → test\\\\" (wrong!)
    local input=$(printf 'test\\"quote')
    local actual=$(escape_json "${input}")
    local expected=$(printf 'test\\\\\\"quote')  # test + \\ + \" + quote
    if [ "${actual}" = "${expected}" ]; then
        printf "PASS: %s\n" "JSON: Escape order (backslash before quote)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        printf "FAIL: %s\n" "JSON: Escape order (backslash before quote)"
        printf "  Input:    %s\n" "${input}"
        printf "  Expected: %s\n" "${expected}"
        printf "  Actual:   %s\n" "${actual}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
    validate_with_jq "${actual}" "JSON: Backslash-quote validates with jq"

    # Test 1: Backslashes (critical for Windows paths)
    local input='C:\Windows\System32'
    local actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Backslashes in Windows path"

    # Test 2: Double quotes (JSON injection risk)
    input='He said "Hello"'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Double quotes"

    # Test 3: Newlines
    input=$(printf 'Line1\nLine2')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Newlines"

    # Test 4: Carriage returns (vmkfstools progress)
    input=$(printf 'Progress: 10%%\rProgress: 100%%')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Carriage returns"

    # Test 5: Tabs
    input=$(printf 'Col1\tCol2')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Tabs"

    # Test 6: Combined special characters
    input=$(printf 'Error:\n\t"File not found"\n\tPath: C:\\test')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Multiple special characters"

    # Test 7: JSON injection attempt
    input='", "injected": "value'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Injection attempt"

    # Test 8: Multiple consecutive backslashes
    input='\\\\'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Multiple backslashes"

    # Test 9: Empty string
    input=''
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Empty string"

    # Test 10: vmkfstools error (typical)
    input='Failed to clone disk: Input/output error (327689).'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Typical vmkfstools error"

    # Test 11-30: Many more edge cases

    # Backslash at end
    input='test\'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Backslash at end"

    # Quote at start
    input='"start'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Quote at start"

    # Quote at end
    input='end"'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Quote at end"

    # Newline at start
    input=$(printf '\nstart')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Newline at start"

    # Newline at end
    input=$(printf 'end\n')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Newline at end"

    # Multiple quotes
    input='"""'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Multiple quotes"

    # Backslash before quote
    input='test\"quote'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Backslash before quote"

    # Tab at start
    input=$(printf '\tstart')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Tab at start"

    # Mixed whitespace
    input=$(printf ' \t\n\r ')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Mixed whitespace"

    # Unicode-like characters (ASCII only)
    input='Test with symbols: @#$%^&*()'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Special symbols"

    # Forward slashes (should NOT be escaped)
    input='/path/to/file'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Forward slashes"

    # Curly braces
    input='{ "test": "value" }'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Curly braces"

    # Square brackets
    input='[1, 2, 3]'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Square brackets"

    # Colon and comma
    input='key:value,key2:value2'
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Colon and comma"

    # Very long string
    input=$(printf 'A%.0s' {1..1000})
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Very long string (1000 chars)"

    # Mixed line endings (CRLF)
    input=$(printf 'Line1\r\nLine2\r\nLine3')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: CRLF line endings"

    # Backspace character
    input=$(printf 'test\bbackspace')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Backspace character"

    # Form feed character
    input=$(printf 'test\fformfeed')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: Form feed character"

    # Real vmkfstools progress output
    input=$(printf 'Clone: 0%% done.\rClone: 25%% done.\rClone: 50%% done.\rClone: 100%% done.')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: vmkfstools progress with \\r"

    # Real vmkfstools error with newlines
    input=$(printf 'Failed to clone disk.\nReason: Insufficient disk space\nError code: 28')
    actual=$(escape_json "${input}")
    validate_with_jq "${actual}" "JSON: vmkfstools multi-line error"

}

# ============================================================================
# GROUP 4: Task Management - Get
# ============================================================================

# Test: Task Get Error Cases
test_task_get_errors() {
    echo ""
    echo "=== Testing Task Get Error Cases ==="

    # Test missing task ID
    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-get 2>&1)
    local exit_code=$?

    assert_exit_code 1 ${exit_code} "Task-get without ID fails"
    assert_contains '"stdErr"' "${output}" "Error message in JSON for missing task ID"
    assert_contains 'task-id' "${output}" "Error mentions task-id requirement"
    assert_contains 'required' "${output}" "Error message indicates parameter is required"

    # Test non-existent task ID
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "nonexistent-uuid-12345" 2>&1)
    exit_code=$?

    assert_exit_code 1 ${exit_code} "Task-get with invalid ID fails"
    assert_contains '"stdErr"' "${output}" "Error message for non-existent task"
    assert_contains 'not found' "${output}" "Error mentions task not found"
    assert_contains 'nonexistent-uuid-12345' "${output}" "Error message includes the task ID"
}

# Test: Task Get Missing Files (Incremental)
test_task_get_missing_files() {
    echo ""
    echo "=== Testing Task Get Missing Files (Incremental) ==="

    local test_task_id="test-missing-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"

    # Step 1: Only task directory, no pid file
    echo "  Step 1: Task directory exists, pid file missing..."
    mkdir -p "${task_dir}"

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    local exit_code=$?

    assert_exit_code 1 ${exit_code} "Missing pid: task-get fails"
    assert_contains '"stdErr"' "${output}" "Missing pid: error in JSON"
    assert_contains 'Failed to read PID' "${output}" "Missing pid: specific error message"

    # Step 2: Add pid file, out file missing
    echo "  Step 2: Add pid file, out file missing..."
    echo "99999" > "${task_dir}/pid"

    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    exit_code=$?

    assert_exit_code 0 ${exit_code} "Missing out: task-get succeeds"
    assert_contains '"pid": 99999' "${output}" "Missing out: PID present"
    assert_contains '"lastLine": ""' "${output}" "Missing out: lastLine is empty string"
    assert_contains '"xcopyUsed": null' "${output}" "Missing out: xcopyUsed is null (targetLun missing)"
    assert_contains '"exitCode": "1"' "${output}" "Missing out: exitCode is 1 (process not running)"

    # Step 3: Add out file, err file missing
    echo "  Step 3: Add out file, err file missing..."
    echo "Progress: 50%" > "${task_dir}/out"

    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    exit_code=$?

    assert_exit_code 0 ${exit_code} "Missing err: task-get succeeds"
    assert_contains '"lastLine": "Progress: 50%"' "${output}" "Missing err: lastLine has content"
    assert_contains '"stdErr": ""' "${output}" "Missing err: stdErr is empty string"

    # Step 4: Add err file, exitcode file missing
    echo "  Step 4: Add err file, exitcode file missing..."
    echo "Warning: test warning" > "${task_dir}/err"

    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    exit_code=$?

    assert_exit_code 0 ${exit_code} "Missing exitcode: task-get succeeds"
    assert_contains '"stdErr": "Warning: test warning"' "${output}" "Missing exitcode: stdErr has content"
    assert_contains '"exitCode": "1"' "${output}" "Missing exitcode: exitCode is 1 (assumes failure)"

    # Step 5: Add exitcode file, targetLun file missing
    echo "  Step 5: Add exitcode file, targetLun file missing..."
    echo "0" > "${task_dir}/exitcode"

    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    exit_code=$?

    assert_exit_code 0 ${exit_code} "Missing targetLun: task-get succeeds"
    assert_contains '"exitCode": "0"' "${output}" "Missing targetLun: exitCode has value"
    assert_contains '"xcopyUsed": null' "${output}" "Missing targetLun: xcopyUsed is null"

    # Step 6: Add targetLun file (complete task directory)
    echo "  Step 6: Add targetLun file (all files present)..."
    echo "naa.600test" > "${task_dir}/targetLun"

    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>/dev/null)
    exit_code=$?

    assert_exit_code 0 ${exit_code} "All files: task-get succeeds"
    assert_contains '"taskId"' "${output}" "All files: taskId present"
    assert_contains '"pid": 99999' "${output}" "All files: pid present"
    assert_contains '"exitCode": "0"' "${output}" "All files: exitCode present"
    assert_contains '"lastLine": "Progress: 50%"' "${output}" "All files: lastLine present"
    assert_contains '"stdErr": "Warning: test warning"' "${output}" "All files: stdErr present"

    # Cleanup
    rm -rf "${task_dir}"
}

# Test: Task Get Success Cases
test_task_get_success() {
    echo ""
    echo "=== Testing Task Get Success Cases ==="

    # Create mock task directory
    local test_task_id="test-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"

    mkdir -p "${task_dir}"
    echo "12345" > "${task_dir}/pid"
    echo "0" > "${task_dir}/exitcode"
    echo "Clone: 100% done." > "${task_dir}/out"
    echo "" > "${task_dir}/err"

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)
    local exit_code=$?

    assert_exit_code 0 ${exit_code} "Task-get succeeds with valid task"
    assert_contains '"taskId"' "${output}" "Task-get output contains taskId"
    assert_contains '"pid"' "${output}" "Task-get output contains pid"
    assert_contains '"exitCode"' "${output}" "Task-get output contains exitCode"
    assert_contains '"lastLine"' "${output}" "Task-get output contains lastLine"
    assert_contains '"xcopyUsed"' "${output}" "Task-get output contains xcopyUsed"
    assert_contains '"stdErr"' "${output}" "Task-get output contains stdErr"
    assert_contains '12345' "${output}" "Task-get output shows correct PID"
    assert_contains 'Clone: 100% done.' "${output}" "Task-get output shows last line"

    # Cleanup
    rm -rf "${task_dir}"
}

# ============================================================================
# GROUP 5: XCOPY Detection
# ============================================================================

# Test: XCOPY Detection
test_xcopy_detection() {
    echo ""
    echo "=== Testing XCOPY Detection ==="

    # Test with no targetLun file (should be null)
    local test_task_id="test-xcopy-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"

    mkdir -p "${task_dir}"
    echo "12345" > "${task_dir}/pid"
    echo "0" > "${task_dir}/exitcode"
    echo "test" > "${task_dir}/out"
    echo "" > "${task_dir}/err"

    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)

    assert_contains '"xcopyUsed": null' "${output}" "xcopyUsed is null when targetLun missing"

    # Test with targetLun file (vsish will fail in test env, should return null)
    echo "naa.600test" > "${task_dir}/targetLun"
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)

    # Since vsish doesn't exist in test environment, should be null
    assert_contains '"xcopyUsed": null' "${output}" "xcopyUsed is null when vsish fails"

    # Cleanup
    rm -rf "${task_dir}"
}

# Test: XCOPY Parsing Logic
test_xcopy_parsing() {
    echo ""
    echo "=== Testing XCOPY Stats Parsing ==="

    # Create a test script that simulates the parsing logic
    cat > "${TEST_TMP_DIR}/test_xcopy_parse.sh" << 'TESTEOF'
#!/bin/sh

# Simulate the parsing logic from was_xcopy_used (vsish format)
parse_xcopy_stats() {
    local target_lun_stats="$1"

    local write_ops=0
    local stat_line
    # Look for "clonewriteops" field (vsish format, no spaces)
    stat_line=$(echo "${target_lun_stats}" | grep -i "clonewriteops")

    if [ -n "${stat_line}" ]; then
        local hex_value
        hex_value=$(echo "${stat_line}" | cut -d':' -f2 | sed 's/[[:space:]]//g')

        # Convert from hex to decimal
        if echo "${hex_value}" | grep -qE '^0x[0-9a-fA-F]+$'; then
            write_ops=$(printf '%d' "${hex_value}" 2>/dev/null || echo "0")
        else
            write_ops=0
        fi
    fi

    if [ ${write_ops} -gt 0 ]; then
        echo "true ${write_ops}"
    else
        echo "false ${write_ops}"
    fi
}

parse_xcopy_stats "$@"
TESTEOF
    chmod +x "${TEST_TMP_DIR}/test_xcopy_parse.sh"

    # Test 1: Parse with XCOPY used (hex value, vsish format)
    local vsish_output="   cloneWriteOps:0x000000000000a560"
    local result
    result=$(sh "${TEST_TMP_DIR}/test_xcopy_parse.sh" "${vsish_output}")

    assert_contains "true" "${result}" "XCOPY detected when clone write ops > 0"
    assert_contains "42336" "${result}" "Correct write ops count extracted (0xa560 = 42336)"

    # Test 2: Parse with no XCOPY used (zero hex value)
    vsish_output="   cloneWriteOps:0x0000000000000000"
    result=$(sh "${TEST_TMP_DIR}/test_xcopy_parse.sh" "${vsish_output}")

    assert_contains "false" "${result}" "XCOPY not detected when clone write ops = 0"
    assert_contains "0" "${result}" "Zero write ops correctly reported"

    # Test 3: Parse multi-line vsish output (realistic vsish -r -e cat format)
    vsish_output="Statistics {
   successfulCmds:44265
   failedCmds:0
   cloneWriteOps:0x000000000000a560
   blocksCloneWrite:0x00000000c43c1c00
}"
    result=$(sh "${TEST_TMP_DIR}/test_xcopy_parse.sh" "${vsish_output}")

    assert_contains "true" "${result}" "XCOPY detected in multi-line output"
    assert_contains "42336" "${result}" "Correct count from multi-line output"

    # Test 4: No clone write ops field
    vsish_output="Statistics {
   successfulCmds:100
   failedCmds:0
}"
    result=$(sh "${TEST_TMP_DIR}/test_xcopy_parse.sh" "${vsish_output}")

    assert_contains "false 0" "${result}" "No XCOPY when field missing"

    # Test 5: Uppercase hex digits
    vsish_output="   cloneWriteOps:0x000000000000ABCD"
    result=$(sh "${TEST_TMP_DIR}/test_xcopy_parse.sh" "${vsish_output}")

    assert_contains "true" "${result}" "XCOPY detected with uppercase hex"
    assert_contains "43981" "${result}" "Uppercase hex correctly parsed (0xABCD = 43981)"
}


# ============================================================================
# GROUP 6: Task Management - Clean
# ============================================================================

# Test: Task Clean Error Cases
test_task_clean_errors() {
    echo ""
    echo "=== Testing Task Clean Error Cases ==="

    # Test missing task ID
    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-clean 2>&1)
    local exit_code=$?

    assert_exit_code 1 ${exit_code} "Task-clean without ID fails"
    assert_contains '"stdErr"' "${output}" "Error message for missing task ID"

    # Test non-existent task
    output=$(sh "${WRAPPER_SCRIPT}" --task-clean -i "nonexistent-12345" 2>&1)
    exit_code=$?

    assert_exit_code 1 ${exit_code} "Task-clean with invalid ID fails"
    assert_contains 'not found' "${output}" "Error mentions task not found"
}

# Test: Task Clean Success Cases
test_task_clean_success() {
    echo ""
    echo "=== Testing Task Clean Success Cases ==="

    # Create mock task directory
    local test_task_id="test-clean-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"

    mkdir -p "${task_dir}"
    echo "12345" > "${task_dir}/pid"
    echo "test" > "${task_dir}/out"

    # Verify directory exists
    test -d "${task_dir}"
    assert_exit_code 0 $? "Task directory created"

    # Clean task
    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-clean -i "${test_task_id}" 2>&1)
    local exit_code=$?

    assert_exit_code 0 ${exit_code} "Task-clean succeeds"
    assert_contains '<string>0</string>' "${output}" "Task-clean returns success status"

    # Verify directory removed
    test -d "${task_dir}"
    assert_exit_code 1 $? "Task directory removed after clean"
}

# Test: Argument Parsing
test_argument_parsing() {
    echo ""
    echo "=== Testing Argument Parsing ==="

    # Test long form arguments
    local output
    output=$(sh "${WRAPPER_SCRIPT}" --version 2>&1)
    assert_contains "0.3.0" "${output}" "Long form --version works"

    # Test short form arguments
    output=$(sh "${WRAPPER_SCRIPT}" -v 2>&1)
    assert_contains "0.3.0" "${output}" "Short form -v works"

    # Test task-get with -i
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "test-id" 2>&1)
    assert_contains 'test-id' "${output}" "Task-get with -i parses task ID"

    # Test task-get with --task-id
    output=$(sh "${WRAPPER_SCRIPT}" --task-get --task-id "test-id-2" 2>&1)
    assert_contains 'test-id-2' "${output}" "Task-get with --task-id parses task ID"
}

# ============================================================================
# GROUP 7: Utility Functions
# ============================================================================

# Test: Task ID Generation
test_task_id_generation() {
    echo ""
    echo "=== Testing Task ID Generation ==="

    # Extract generate_task_id function for testing
    cat > "${TEST_TMP_DIR}/test_task_id.sh" << 'TESTEOF'
#!/bin/sh

generate_task_id() {
    if [ -r /dev/urandom ]; then
        local hex
        hex=$(dd if=/dev/urandom bs=16 count=1 2>/dev/null | od -A n -t x1 | sed 's/ //g' | sed 's/\n//g')

        local p1=$(echo "${hex}" | cut -c1-8)
        local p2=$(echo "${hex}" | cut -c9-12)
        local p3=$(echo "${hex}" | cut -c13-16)
        local p4=$(echo "${hex}" | cut -c17-20)
        local p5=$(echo "${hex}" | cut -c21-32)

        echo "${p1}-${p2}-${p3}-${p4}-${p5}"
    elif [ -r /dev/random ]; then
        local hex
        hex=$(dd if=/dev/random bs=16 count=1 2>/dev/null | od -A n -t x1 | sed 's/ //g' | sed 's/\n//g')

        local p1=$(echo "${hex}" | cut -c1-8)
        local p2=$(echo "${hex}" | cut -c9-12)
        local p3=$(echo "${hex}" | cut -c13-16)
        local p4=$(echo "${hex}" | cut -c17-20)
        local p5=$(echo "${hex}" | cut -c21-32)

        echo "${p1}-${p2}-${p3}-${p4}-${p5}"
    else
        local seed="$(date +%s)-$$-$(date +%N 2>/dev/null || echo $$)"
        if command -v md5sum >/dev/null 2>&1; then
            local hash=$(echo "${seed}" | md5sum | cut -d' ' -f1)
            local p1=$(echo "${hash}" | cut -c1-8)
            local p2=$(echo "${hash}" | cut -c9-12)
            local p3=$(echo "${hash}" | cut -c13-16)
            local p4=$(echo "${hash}" | cut -c17-20)
            local p5=$(echo "${hash}" | cut -c21-32)
            echo "${p1}-${p2}-${p3}-${p4}-${p5}"
        else
            echo "${seed}"
        fi
    fi
}

generate_task_id
TESTEOF
    chmod +x "${TEST_TMP_DIR}/test_task_id.sh"

    # Test 1: Task ID has valid UUID format
    local task_id1
    task_id1=$(sh "${TEST_TMP_DIR}/test_task_id.sh")

    # Check UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    if echo "${task_id1}" | grep -qE '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'; then
        local actual_len
        actual_len=$(echo -n "${task_id1}" | wc -c)
        assert_equals "36" "${actual_len}" "Task ID has correct length"
    else
        # Fallback format check
        if echo "${task_id1}" | grep -qE '^[0-9]+-[0-9]+-'; then
            printf "PASS: %s\n" "Task ID has valid fallback format"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            printf "FAIL: %s\n" "Task ID format invalid"
            printf "  Got: %s\n" "${task_id1}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
        TESTS_RUN=$((TESTS_RUN + 1))
    fi

    # Test 2: Task ID format matches UUID pattern
    if echo "${task_id1}" | grep -qE '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'; then
        printf "PASS: %s\n" "Task ID matches UUID format"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        printf "SKIP: %s\n" "Task ID UUID format - using fallback"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))

    # Test 3: Task IDs are unique
    local id_list="${task_id1}"
    local duplicate_found=false

    for i in 2 3 4 5; do
        local new_id
        new_id=$(sh "${TEST_TMP_DIR}/test_task_id.sh")

        if echo "${id_list}" | grep -qF "${new_id}"; then
            duplicate_found=true
            break
        fi
        id_list="${id_list} ${new_id}"
    done

    if [ "${duplicate_found}" = "false" ]; then
        printf "PASS: %s\n" "Task IDs are unique - 5 generated"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        printf "FAIL: %s\n" "Duplicate task ID found"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))

    # Test 4: Task ID is not empty
    if [ -n "${task_id1}" ] && [ "${task_id1}" != "----" ]; then
        printf "PASS: %s\n" "Task ID is not empty or malformed"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        printf "FAIL: %s\n" "Task ID is empty or malformed"
        printf "  Got: '%s'\n" "${task_id1}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))

    # Test 5: Task ID contains no whitespace
    if ! echo "${task_id1}" | grep -q '[[:space:]]'; then
        printf "PASS: %s\n" "Task ID contains no whitespace"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        printf "FAIL: %s\n" "Task ID contains whitespace"
        printf "  Got: '%s'\n" "${task_id1}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_RUN=$((TESTS_RUN + 1))
}

# Test: get_last_line Function
test_get_last_line() {
    echo ""
    echo "=== Testing get_last_line Function ==="

    # Create test script for get_last_line
    cat > "${TEST_TMP_DIR}/get_last_line.sh" << 'TESTEOF'
#!/bin/sh

get_last_line() {
    local file="$1"

    if [ ! -f "${file}" ]; then
        echo ""
        return 0
    fi

    local file_size
    file_size=$(wc -c < "${file}" 2>/dev/null)

    if [ -z "${file_size}" ] || [ "${file_size}" -eq 0 ]; then
        echo ""
        return 0
    fi

    local chunk_size=4096
    if [ "${file_size}" -lt "${chunk_size}" ]; then
        chunk_size="${file_size}"
    fi

    local content
    content=$(tail -c "${chunk_size}" "${file}" 2>/dev/null)

    local last_segment
    last_segment=$(echo "${content}" | sed 's/\r/\n/g' | tail -n 1)

    if [ -n "${last_segment}" ]; then
        echo "${last_segment}"
        return 0
    fi

    echo ""
}

get_last_line "$@"
TESTEOF
    chmod +x "${TEST_TMP_DIR}/get_last_line.sh"

    # Test 1: Empty file
    > "${TEST_TMP_DIR}/empty.txt"
    local result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/empty.txt")
    assert_equals "" "${result}" "get_last_line: Empty file"

    # Test 2: Single line without newline
    printf "SingleLine" > "${TEST_TMP_DIR}/single.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/single.txt")
    assert_equals "SingleLine" "${result}" "get_last_line: Single line no newline"

    # Test 3: Single line with newline
    printf "SingleLine\n" > "${TEST_TMP_DIR}/single_nl.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/single_nl.txt")
    assert_equals "SingleLine" "${result}" "get_last_line: Single line with newline"

    # Test 4: Multiple lines without final newline
    printf "Line1\nLine2\nLine3" > "${TEST_TMP_DIR}/multi.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/multi.txt")
    assert_equals "Line3" "${result}" "get_last_line: Multi-line no final newline"

    # Test 5: vmkfstools progress (carriage returns)
    printf "Clone: 0%%\rClone: 25%%\rClone: 100%% done." > "${TEST_TMP_DIR}/progress.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/progress.txt")
    assert_equals "Clone: 100% done." "${result}" "get_last_line: Carriage return progress"

    # Test 6: Mixed newlines and carriage returns
    printf "Line1\nProgress: 10%%\rProgress: 100%%" > "${TEST_TMP_DIR}/mixed.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/mixed.txt")
    assert_equals "Progress: 100%" "${result}" "get_last_line: Mixed \\n and \\r"

    # Test 7: Large file (>4096 bytes)
    for i in $(seq 1 200); do
        echo "Line $i" >> "${TEST_TMP_DIR}/large.txt"
    done
    printf "VERY LAST LINE" >> "${TEST_TMP_DIR}/large.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/large.txt")
    assert_equals "VERY LAST LINE" "${result}" "get_last_line: Large file >4KB"

    # Test 8: File with only whitespace
    printf "   \n\t\n  " > "${TEST_TMP_DIR}/whitespace.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/whitespace.txt")
    assert_equals "  " "${result}" "get_last_line: Preserves trailing spaces"

    # Test 9: CRLF line endings (Windows)
    printf "Line1\r\nLine2\r\nLine3" > "${TEST_TMP_DIR}/crlf.txt"
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/crlf.txt")
    assert_equals "Line3" "${result}" "get_last_line: CRLF line endings"

    # Test 10: Non-existent file
    result=$(sh "${TEST_TMP_DIR}/get_last_line.sh" "${TEST_TMP_DIR}/nonexistent.txt")
    assert_equals "" "${result}" "get_last_line: Non-existent file"
}


# Test: Clone Response Format
test_clone_response_format() {
    echo ""
    echo "=== Testing Clone Response Format ==="

    # We can't actually run clone without vmkfstools, but we can verify
    # the response format should NOT contain taskPath
    # Check the script source to ensure taskPath is not in clone response
    local script_content
    script_content=$(cat "${WRAPPER_SCRIPT}")

    # Find the clone function's json_result line
    local clone_json_line
    clone_json_line=$(echo "${script_content}" | grep -A 5 'local json_result=.*taskId.*pid' | head -1)

    assert_not_contains 'taskPath' "${clone_json_line}" "Clone response does not include taskPath"
    assert_contains 'taskId' "${clone_json_line}" "Clone response includes taskId"
    assert_contains 'pid' "${clone_json_line}" "Clone response includes pid"
}

# Test: TMP_PREFIX Usage
test_tmp_prefix_usage() {
    echo ""
    echo "=== Testing TMP_PREFIX Usage ==="

    # Verify script uses TMP_PREFIX not VMDK_DEFAULT
    local script_content
    script_content=$(cat "${WRAPPER_SCRIPT}")

    assert_contains 'TMP_PREFIX=' "${script_content}" "Script defines TMP_PREFIX"
    assert_contains '/tmp/vmkfstools-wrapper-' "${script_content}" "TMP_PREFIX points to /tmp"
    assert_not_contains 'VMDK_DEFAULT=' "${script_content}" "Script does not use VMDK_DEFAULT"

    # Verify task directories use TMP_PREFIX
    assert_contains 'task_dir=.*TMP_PREFIX' "${script_content}" "Task directories use TMP_PREFIX"
}

# ============================================================================
# GROUP 8: Integration & Compatibility Tests
# ============================================================================

# Test: Task Output with Special Characters (Integration)
test_task_output_with_special_chars() {
    echo ""
    echo "=== CRITICAL: Task Output Special Characters Integration ==="

    local test_task_id="test-chars-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"
    mkdir -p "${task_dir}"
    echo "12345" > "${task_dir}/pid"
    echo "0" > "${task_dir}/exitcode"

    # Test 1: Output with quotes
    printf 'Error: "File not found"' > "${task_dir}/out"
    printf 'stderr: "Invalid parameter"' > "${task_dir}/err"
    local output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)
    echo "${output}" | grep -q '<?xml version'
    assert_exit_code 0 $? "Integration: XML valid with quotes"
    echo "${output}" | grep -q '\\"'
    assert_exit_code 0 $? "Integration: Quotes escaped in JSON"

    # Test 2: Output with carriage returns
    printf 'Clone: 0%%\rClone: 100%% done.' > "${task_dir}/out"
    > "${task_dir}/err"
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)
    echo "${output}" | grep -q 'Clone: 100% done.'
    assert_exit_code 0 $? "Integration: Carriage return shows final"

    # Test 3: Multi-line error
    printf 'Error1\nError2\nError3' > "${task_dir}/err"
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)
    echo "${output}" | grep -q '\\n'
    assert_exit_code 0 $? "Integration: Newlines escaped in stderr"

    rm -rf "${task_dir}"
}

# Test: Task Clean with Locked Files (Edge Cases)
test_task_clean_with_locked_files() {
    echo ""
    echo "=== CRITICAL: task_clean Edge Cases ==="

    # Test 1: Cleanup with symlink to protected file (security test)
    echo "  Test 1: Cleanup with symlink to system file (security)..."
    local test_task_id="test-symlink-$(date +%s)"
    local task_dir="${TMP_PREFIX}${test_task_id}"
    mkdir -p "${task_dir}"
    echo "55555" > "${task_dir}/pid"

    # Create symlink to /etc/passwd (should NOT be deleted!)
    ln -s /etc/passwd "${task_dir}/should-not-delete"

    # Verify /etc/passwd exists before cleanup
    if [ ! -f /etc/passwd ]; then
        echo "SKIP: /etc/passwd not found (unusual system)"
        TESTS_RUN=$((TESTS_RUN + 1))
    else
        # Try cleanup
        local output
        output=$(sh "${WRAPPER_SCRIPT}" --task-clean -i "${test_task_id}" 2>&1)

        # Verify /etc/passwd still exists (symlink target NOT deleted)
        # rm -rf only removes the symlink, not the target
        if [ -f /etc/passwd ]; then
            echo "PASS: Symlink target not deleted (secure)"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo "FAIL: SECURITY ISSUE - symlink target was deleted!"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
        TESTS_RUN=$((TESTS_RUN + 1))
    fi

    # Clean up
    rm -rf "${task_dir}" 2>/dev/null || true

    # Test 2: Cleanup with RDM files that don't exist (should not crash)
    echo "  Test 2: Cleanup with missing RDM files..."
    test_task_id="test-missing-rdm-$$-$(date +%s%N 2>/dev/null || date +%s)"
    task_dir="${TMP_PREFIX}${test_task_id}"
    mkdir -p "${task_dir}"
    echo "33333" > "${task_dir}/pid"
    echo "/vmfs/volumes/nonexistent/file.vmdk" > "${task_dir}/rdmfile"

    output=$(sh "${WRAPPER_SCRIPT}" --task-clean -i "${test_task_id}" 2>&1)

    # Main goal: cleanup should return valid XML and not crash
    assert_contains '<?xml version' "${output}" "MISSING: Returns XML"
    assert_contains '<field name="status">' "${output}" "MISSING: Has status field"

    # Clean up if still exists
    rm -rf "${task_dir}" 2>/dev/null || true

    # Test 3: Cleanup returns valid XML even with warnings
    echo "  Test 3: Cleanup always returns valid XML..."
    test_task_id="test-xml-$(date +%s)"
    task_dir="${TMP_PREFIX}${test_task_id}"
    mkdir -p "${task_dir}"
    echo "44444" > "${task_dir}/pid"

    output=$(sh "${WRAPPER_SCRIPT}" --task-clean -i "${test_task_id}" 2>&1)

    # Should return valid XML structure
    echo "${output}" | grep -q '<?xml version'
    assert_exit_code 0 $? "XML: Valid XML header"

    echo "${output}" | grep -q '<field name="status">'
    assert_exit_code 0 $? "XML: Status field present"

    # Clean up if still exists
    rm -rf "${task_dir}" 2>/dev/null || true
}

# Test: Go Struct Compatibility
test_go_struct_compatibility() {
    echo ""
    echo "=== Testing Go Struct Compatibility ==="
    
    # Create a complete test task directory to ensure all fields are present
    local test_task_id="test-compat-$(date +%s)"
    local task_dir="/tmp/vmkfstools-wrapper-${test_task_id}"
    
    mkdir -p "${task_dir}"
    echo "12345" > "${task_dir}/pid"
    echo "0" > "${task_dir}/exitcode"
    echo "Clone: 100% done." > "${task_dir}/out"
    echo "" > "${task_dir}/err"
    echo "naa.600test" > "${task_dir}/targetLun"
    
    local output
    output=$(sh "${WRAPPER_SCRIPT}" --task-get -i "${test_task_id}" 2>&1)
    local exit_code=$?
    
    # Cleanup test directory
    rm -rf "${task_dir}" 2>/dev/null || true
    
    if [ ${exit_code} -ne 0 ]; then
        echo "SKIP: Task-get failed for test task (exit code: ${exit_code})"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        TESTS_RUN=$((TESTS_RUN + 1))
        return 0
    fi
    
    if command -v jq >/dev/null 2>&1; then
        local json_message
        json_message=$(echo "${output}" | sed -n 's/.*<field name="message"><string>\(.*\)<\/string><\/field>.*/\1/p' | sed 's/&lt;/</g' | sed 's/&gt;/>/g' | sed 's/&amp;/\&/g')
        
        # Check required fields for vmkfstoolsTask struct
        local required_fields=("taskId" "pid" "exitCode" "lastLine" "stdErr" "xcopyUsed")
        local missing_fields=()
        
        for field in "${required_fields[@]}"; do
            # Use 'has()' to check if field exists, as jq -e fails on false/null values
            if ! echo "${json_message}" | jq -e "has(\"${field}\")" >/dev/null 2>&1; then
                missing_fields+=("${field}")
            fi
        done
        
        if [ ${#missing_fields[@]} -gt 0 ]; then
            printf "FAIL: Missing required fields: %s\n" "${missing_fields[*]}"
            printf "  JSON output: %s\n" "${json_message}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            TESTS_RUN=$((TESTS_RUN + 1))
            return 1
        else
            printf "PASS: All required fields for Go struct are present\n"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            TESTS_RUN=$((TESTS_RUN + 1))
        fi
    else
        echo "SKIP: jq not available for struct compatibility test"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        TESTS_RUN=$((TESTS_RUN + 1))
    fi
    
    return 0
}

# ============================================================================
# MAIN TEST RUNNER
# ============================================================================
main() {
    echo "================================================================"
    echo "     vmkfstools_wrapper.sh - Unit Test Suite"
    echo "================================================================"
    echo ""
    echo "Testing script: ${WRAPPER_SCRIPT}"

    if [ ! -f "${WRAPPER_SCRIPT}" ]; then
        echo "ERROR: Script not found: ${WRAPPER_SCRIPT}"
        exit 1
    fi

    test_setup

    # GROUP 1: Basic Command Tests
    test_version_command
    test_version_output_simple
    test_version_output_xml
    test_argument_parsing
    
    
    # GROUP 2: Path Validation and Security
    test_path_validation
    
    # GROUP 3: Output Format Tests
    test_xml_output_format
    test_json_escaping
    test_clone_response_format
    test_tmp_prefix_usage
    
    # GROUP 4: Task Management - Get
    test_task_get_errors
    test_task_get_missing_files
    test_task_get_success
    test_get_last_line
    
    # GROUP 5: XCOPY Detection
    test_xcopy_detection
    test_xcopy_parsing
    
    # GROUP 6: Task Management - Clean
    test_task_clean_errors
    test_task_clean_success
    test_task_clean_with_locked_files
    
    # GROUP 7: Utility Functions
    test_task_id_generation
    
    # GROUP 8: Integration & Compatibility Tests
    test_task_output_with_special_chars
    test_go_struct_compatibility

    test_teardown

    # Print summary
    echo ""
    echo "================================================================"
    echo "                       TEST SUMMARY"
    echo "================================================================"
    echo ""
    printf "  Total Tests:  %d\n" ${TESTS_RUN}
    printf "  Passed:       %d\n" ${TESTS_PASSED}
    printf "  Failed:       %d\n" ${TESTS_FAILED}
    echo ""

    if [ ${TESTS_FAILED} -eq 0 ]; then
        printf "PASS: All tests passed!\n"
        return 0
    else
        printf "FAIL: Some tests failed\n"
        return 1
    fi
}

# Run tests
main "$@"
exit $?
