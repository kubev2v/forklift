#!/bin/sh

set -e

TMP_PREFIX="/tmp/vmkfstools-wrapper-"
SCRIPT_VERSION="0.3.0"
LOGIN_TAG="vmkfstools-wrapper"

xml_output() {
    local status="$1"
    local message="$2"
    cat <<EOF
<?xml version="1.0" ?>
<output xmlns="http://www.vmware.com/Products/ESX/5.0/esxcli/">
    <structure typeName="result">
        <field name="status"><string>${status}</string></field>
        <field name="message"><string>${message}</string></field>
    </structure>
</output>
EOF
}

log_info() {
    local message="$1"
    logger -t "${LOGIN_TAG}" -p INFO "${message}"
}

log_error() {
    local message="$1"
    logger -t "${LOGIN_TAG}" -p ERROR "${message}"
}

log_warning() {
    local message="$1"
    logger -t "${LOGIN_TAG}" -p WARN "${message}"
}

# Escape special characters for JSON strings
escape_json() {
    local str="$1"

    # IMPORTANT: Escape backslashes FIRST, before other escapes
    # Otherwise we'll double-escape the escape sequences!

    # 1. Escape backslashes first ( \ -> \\ )
    str="${str//\\/\\\\}"

    # 2. Escape double quotes ( " -> \" )
    str="${str//\"/\\\"}"

    # 3. Escape newlines ( literal newline -> \n )
    str="${str//$'\n'/\\n}"

    # 4. Escape tabs ( literal tab -> \t )
    str="${str//$'\t'/\\t}"

    # 5. Escape carriage returns ( \r -> \r )
    str="${str//$'\r'/\\r}"

    # 6. Escape backspace ( optional, \b -> \b )
    str="${str//$'\b'/\\b}"

    # 7. Escape form feed ( optional, \f -> \f )
    str="${str//$'\f'/\\f}"

    printf '%s' "$str"
}

validate_path() {
    local path="$1"

    log_info "vmkfstools-wrapper version ${SCRIPT_VERSION} validating path: ${path}"

    # CRITICAL: Normalize FIRST before security checks
    path=$(echo "${path}" | sed 's|//*|/|g')
    path=$(echo "${path}" | sed 's|/\./|/|g')
    path=$(echo "${path}" | sed 's|/\+$||')

    # Check for path traversal BEFORE prefix check
    case "${path}" in
        *..*|*/../*|*/..*|*/..)
            log_error "Path traversal detected: ${path}"
            echo "Invalid path detected: ${path}"
            return 1
            ;;
    esac

    # Check for allowed prefixes (after normalization and traversal check)
    case "${path}" in
        /vmfs/volumes/*|/vmfs/devices/disks/*|/tmp/*)
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

# vmkfstools uses \r (carriage return) for progress updates on same line
# Read last 4096 bytes and extract text after final \r
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

    # Replace \r with \n using sed (BusyBox compatible)
    local last_segment
    last_segment=$(echo "${content}" | sed 's/\r/\n/g' | tail -n 1)

    if [ -n "${last_segment}" ]; then
        echo "${last_segment}"
        return 0
    fi

    echo ""
}

# Generate UUID-compatible ID for ESXi
# Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
generate_task_id() {
    local random_dev
    if [ -r /dev/urandom ]; then
        random_dev=/dev/urandom
    elif [ -r /dev/random ]; then
        random_dev=/dev/random
    else
        return 1
    fi

    local hex
    hex=$(dd if="$random_dev" bs=16 count=1 2>/dev/null | od -A n -t x1 | sed 's/ //g' | sed 's/\n//g')

    echo ${hex:0:8}-${hex:8:4}-${hex:12:4}-${hex:16:4}-${hex:20:12}
}

clone() {
    local source_vmdk="$1"
    local target_lun="$2"

    log_info "Validating source VMDK path..."
    local source
    source=$(validate_path "${source_vmdk}") || {
        xml_output "1" "Path validation error: ${source}"
        return 1
    }
    log_info "Source VMDK path validation passed"

    log_info "Validating target LUN path..."
    local target
    target=$(validate_path "${target_lun}") || {
        xml_output "1" "Path validation error: ${target}"
        return 1
    }
    log_info "Target LUN path validation passed"

    local task_id
    task_id=$(generate_task_id)

    local task_dir="${TMP_PREFIX}${task_id}"

    # Capture exit code BEFORE any other commands to detect ENOSPC correctly
    mkdir -p "${task_dir}"
    local mkdir_exit=$?

    if [ ${mkdir_exit} -ne 0 ]; then
        if [ ${mkdir_exit} -eq 28 ]; then
            xml_output "28" "Error running subprocess: free space is low: check /var/log/vmkernel.log for more details"
        else
            xml_output "1" "Failed to create task directory: ${task_dir} (exit code: ${mkdir_exit})"
        fi
        return 1
    fi

    if [ ! -d "${task_dir}" ] || [ ! -w "${task_dir}" ]; then
        xml_output "1" "Task directory created but not accessible: ${task_dir}"
        rm -rf "${task_dir}" 2>/dev/null
        return 1
    fi

    chmod 750 "${task_dir}"

    local rdmfile="${source_vmdk}-rdmdisk-$$"
    local stdout_file="${task_dir}/out"
    local stderr_file="${task_dir}/err"
    local exitcode_file="${task_dir}/exitcode"

    local vmkfstools_cmdline="trap 'echo -n \$? > ${exitcode_file}' EXIT; /bin/vmkfstools -i '${source}' -d rdm:'${target}' '${rdmfile}'"

    log_info "about to run ${vmkfstools_cmdline}"

    (
        # setsid creates new session 
        setsid /bin/sh -c "${vmkfstools_cmdline}" \
            < /dev/null \
            > "${stdout_file}" \
            2> "${stderr_file}" &

        local task_pid=$!

        if ! kill -0 ${task_pid} 2>/dev/null; then
            log_error "Process ${task_pid} failed to start or died immediately"
            xml_output "1" "Failed to start vmkfstools process"
            exit 1
        fi

        # Atomic file creation: write to temp then rename
        echo "${task_pid}" > "${task_dir}/pid.tmp" && mv "${task_dir}/pid.tmp" "${task_dir}/pid"
        echo "${rdmfile}" > "${task_dir}/rdmfile.tmp" && mv "${task_dir}/rdmfile.tmp" "${task_dir}/rdmfile"
        local target_lun_basename
        target_lun_basename=$(basename "${target}")
        echo "${target_lun_basename}" > "${task_dir}/targetLun.tmp" && mv "${task_dir}/targetLun.tmp" "${task_dir}/targetLun"

        local json_result="{\"taskId\": \"${task_id}\", \"pid\": ${task_pid}, \"xcopyUsed\": false}"
        xml_output "0" "${json_result}"
    ) || {
        local error_msg="Error running subprocess"
        xml_output "1" "${error_msg}"
        return 1
    }
}

extract_rdmdisk_file() {
    local rdm_file="$1"

    if [ ! -f "${rdm_file}" ]; then
        return 1
    fi

    local rdmdisk_filename
    rdmdisk_filename=$(grep 'VMFSRDM' "${rdm_file}" | sed -n 's/.*VMFSRDM "\([^"]*\)".*/\1/p')

    if [ -n "${rdmdisk_filename}" ]; then
        local rdm_dir
        rdm_dir=$(dirname "${rdm_file}")
        echo "${rdm_dir}/${rdmdisk_filename}"
        return 0
    fi

    return 1
}

was_xcopy_used() {
    local target_lun="$1"
    # Remove whitespace 
    target_lun=$(echo "${target_lun}" | sed 's/[[:space:]]//g')

    local stats_path="/storage/scsifw/devices/${target_lun}/stats"

    local target_lun_stats
    target_lun_stats=$(vsish -r -e cat "${stats_path}" 2>&1)
    local vsish_exit=$?

    if [ ${vsish_exit} -ne 0 ]; then
        log_error "Error: Unable to read XCOPY stats for device ${target_lun} with path ${stats_path}"
        return 1
    fi

    log_info "vsish stats ${target_lun_stats}"

    local write_ops=0
    local stat_line
    # Look for "clonewriteops" field (vsish format, no spaces)
    # vsish -r -e cat returns format like: cloneWriteOps:0x0000000000009871
    stat_line=$(echo "${target_lun_stats}" | grep -i "clonewriteops")

    if [ -n "${stat_line}" ]; then
        local hex_value
        # Extract hex value after colon and remove whitespace
        hex_value=$(echo "${stat_line}" | cut -d':' -f2 | sed 's/[[:space:]]//g')

        # Convert from hex to decimal using printf
        # hex_value looks like: 0x0000000000009871
        if echo "${hex_value}" | grep -qE '^0x[0-9a-fA-F]+$'; then
            # Use printf to convert hex to decimal (BusyBox compatible)
            write_ops=$(printf '%d' "${hex_value}" 2>/dev/null || echo "0")
        else
            log_error "Error: Unable to parse hex statistic: ${hex_value}"
            write_ops=0
        fi
    fi

    if [ ${write_ops} -gt 0 ]; then
        echo "true ${write_ops}"
    else
        echo "false ${write_ops}"
    fi
}

task_get() {
    local task_id="$1"

    if [ -z "${task_id}" ]; then
        local json_result="{\"stdErr\": \"-i (task-id) is required for task-get\", \"xcopyUsed\": false}"
        xml_output "1" "${json_result}"
        return 1
    fi

    local task_dir="${TMP_PREFIX}${task_id}"

    if [ ! -d "${task_dir}" ]; then
        local json_result="{\"taskId\": \"${task_id}\", \"stdErr\": \"Task directory ${task_dir} not found\", \"xcopyUsed\": false}"
        xml_output "1" "${json_result}"
        return 1
    fi

    local pid
    pid=$(cat "${task_dir}/pid" 2>/dev/null) || {
        local json_result="{\"taskId\": \"${task_id}\", \"stdErr\": \"Failed to read PID\", \"xcopyUsed\": false}"
        xml_output "1" "${json_result}"
        return 1
    }

    local last_line
    last_line=$(get_last_line "${task_dir}/out")

    local stderr_content
    stderr_content=$(cat "${task_dir}/err" 2>/dev/null || echo "")

    local exitcode
    exitcode=$(cat "${task_dir}/exitcode" 2>/dev/null || echo "")

    # Check if process is still running
    # If exitcode is empty, check if the PID is still alive
    if [ -z "${exitcode}" ]; then
        if ! kill -0 "${pid}" 2>/dev/null; then
            # Process is not running but no exitcode file - process may have died
            # Try to read exitcode one more time in case it was just written
            exitcode=$(cat "${task_dir}/exitcode" 2>/dev/null || echo "")
            if [ -z "${exitcode}" ]; then
                # Process died without writing exitcode - assume failure
                log_warning "Process ${pid} is not running but no exitcode file found, assuming failure"
                exitcode="1"
            fi
        fi
    fi

    local xcopy_used=""
    local target_lun=""
    if [ -f "${task_dir}/targetLun" ]; then
        target_lun=$(cat "${task_dir}/targetLun" 2>/dev/null)
        if [ -n "${target_lun}" ]; then
            # Try to determine xcopy usage, return null if vsish fails
            local xcopy_result
            # Don't redirect stderr - was_xcopy_used logs to stderr, we only want stdout
            xcopy_result=$(was_xcopy_used "${target_lun}")
            if [ $? -eq 0 ]; then
                local xcopy_bool
                xcopy_bool=$(echo "${xcopy_result}" | cut -d' ' -f1)

                log_info "XCOPY used: ${xcopy_bool}"
                xcopy_used="${xcopy_bool}"
            else
                log_warning "Failed to determine xcopy usage, returning null"
            fi
        fi
    else
        log_warning "Failed to determine xcopy usage: targetLun file not found, returning null"
    fi

    local escaped_last_line
    escaped_last_line=$(escape_json "${last_line}")

    local escaped_stderr
    escaped_stderr=$(escape_json "${stderr_content}")  

    local json_result
    # Format xcopyUsed: use null if empty, otherwise use the boolean value
    local xcopy_used_json
    if [ -z "${xcopy_used}" ]; then
        xcopy_used_json="null"
    else
        xcopy_used_json="${xcopy_used}"
    fi
    
    if [ -n "${exitcode}" ]; then
        json_result="{\"taskId\": \"${task_id}\", \"pid\": ${pid}, \"exitCode\": \"${exitcode}\", \"lastLine\": \"${escaped_last_line}\", \"xcopyUsed\": ${xcopy_used_json}, \"stdErr\": \"${escaped_stderr}\"}"
    else
        json_result="{\"taskId\": \"${task_id}\", \"pid\": ${pid}, \"exitCode\": \"\", \"lastLine\": \"${escaped_last_line}\", \"xcopyUsed\": ${xcopy_used_json}, \"stdErr\": \"${escaped_stderr}\"}"
    fi

    xml_output "0" "${json_result}"
}

task_clean() {
    local task_id="$1"

    if [ -z "${task_id}" ]; then
        local json_result="{\"stdErr\": \"-i (task-id) is required for task-clean\", \"xcopyUsed\": false}"
        xml_output "1" "${json_result}"
        return 1
    fi

    local task_dir="${TMP_PREFIX}${task_id}"
    validate_path "${task_dir}" >/dev/null 2>&1 || {
        xml_output "1" "Path validation error: ${task_dir}"
        return 1
    }

    if [ ! -d "${task_dir}" ]; then
        local json_result="{\"taskId\": \"${task_id}\", \"stdErr\": \"Task directory ${task_dir} not found\", \"xcopyUsed\": false}"
        xml_output "1" "${json_result}"
        return 1
    fi

    local rdmfile_path="${task_dir}/rdmfile"

    # Don't fail cleanup if RDM removal fails
    if [ -f "${rdmfile_path}" ]; then
        local rdmfile
        rdmfile=$(cat "${rdmfile_path}" 2>/dev/null)

        if [ -n "${rdmfile}" ] && [ -f "${rdmfile}" ]; then
            local rdmdisk_file
            rdmdisk_file=$(extract_rdmdisk_file "${rdmfile}")

            if [ -n "${rdmdisk_file}" ] && [ -f "${rdmdisk_file}" ]; then
                log_info "removing rdmdisk file ${rdmdisk_file}"
                if rm -f "${rdmdisk_file}" 2>/dev/null; then
                    log_info "removed rdmdisk file ${rdmdisk_file}"
                else
                    log_warning "failed to remove ${rdmdisk_file}"
                fi
            fi

            log_info "removing rdmfile ${rdmfile}"
            if rm -f "${rdmfile}" 2>/dev/null; then
                log_info "removed rdmfile ${rdmfile}"
            else
                log_warning "failed to remove ${rdmfile}"
            fi
        fi
    fi

    if rm -rf "${task_dir}" 2>/dev/null; then
        log_info "removed task directory ${task_dir}"
        xml_output "0" ""
        return 0
    fi
    log_warning "failed to remove task directory ${task_dir}, trying to clean up directory contents"
    log_error "Directory contents: $(ls -la "${task_dir}" 2>&1 || echo 'cannot list')"

    find "${task_dir}" -type f -delete 2>/dev/null
    if rmdir "${task_dir}" 2>/dev/null; then
        log_info "removed task directory ${task_dir} after cleanup"
        xml_output "0" ""
        return 0
    fi
    log_error "failed to remove task directory ${task_dir} after cleanup"
    xml_output "1" "failed to remove task directory ${task_dir} after cleanup"
    return 1
}

version() {
    if [ "${OUTPUT_FORMAT}" = "simple" ]; then
        echo "${SCRIPT_VERSION}"
    else
        local json_result="{\"version\": \"${SCRIPT_VERSION}\"}"
        xml_output "0" "${json_result}"
    fi
}

main() {
    if [ -n "${SSH_ORIGINAL_COMMAND}" ]; then
        log_info "Received SSH_ORIGINAL_COMMAND: ${SSH_ORIGINAL_COMMAND}"
        # Safer than eval but still has word splitting issues
        set -- ${SSH_ORIGINAL_COMMAND}
    fi

    local do_clone=false
    local do_task_get=false
    local do_task_clean=false
    local do_version=false
    local source_vmdk=""
    local target_lun=""
    local task_id=""
    OUTPUT_FORMAT="xml"

    while [ $# -gt 0 ]; do
        case "$1" in
            --clone)
                do_clone=true
                shift
                ;;
            -s|--source-vmdk)
                source_vmdk="$2"
                shift 2
                ;;
            -t|--target-lun)
                target_lun="$2"
                shift 2
                ;;
            --task-get)
                do_task_get=true
                shift
                ;;
            --task-clean)
                do_task_clean=true
                shift
                ;;
            -i|--task-id)
                task_id="$2"
                shift 2
                ;;
            -v|--version)
                do_version=true
                shift
                ;;
            --output)
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                xml_output "1" "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    local exit_code=0

    if [ "${do_version}" = "true" ]; then
        version || exit_code=$?
    fi

    if [ "${do_clone}" = "true" ] && [ -n "${source_vmdk}" ] && [ -n "${target_lun}" ]; then
        clone "${source_vmdk}" "${target_lun}" || exit_code=$?
    elif [ "${do_task_get}" = "true" ]; then
        task_get "${task_id}" || exit_code=$?
    elif [ "${do_task_clean}" = "true" ]; then
        task_clean "${task_id}" || exit_code=$?
    fi

    exit ${exit_code}
}

main "$@"
