#!/bin/bash
#
# xcopy-validate.sh — Validate ESXi VAAI/XCOPY readiness for a datastore
#
# Connects to an ESXi host via SSH and checks all settings that can
# prevent XCOPY (VAAI Full Copy) offload from working.
#
# Usage:
#   ./xcopy-validate.sh --host <esxi> --user root --password <pass> --datastore <name>
#   ./xcopy-validate.sh --host <esxi> --user root --password <pass> --vmdk /vmfs/volumes/DS/vm/vm.vmdk
#   ./xcopy-validate.sh --host <esxi> --user root --password <pass> --all-datastores
#
# Credentials can also come from environment variables:
#   VMWARE_HOST, VMWARE_USER, VMWARE_PASSWORD

set -euo pipefail

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Global counters (aggregate across all devices)
GLOBAL_PASS=0
GLOBAL_WARN=0
GLOBAL_FAIL=0

# Per-device counters (reset each check_device call)
DEV_PASS=0
DEV_WARN=0
DEV_FAIL=0

# Set by check_device — non-empty if iSCSI transport detected
IS_ISCSI=""

FIX_COMMANDS=""

# Per-device summary rows for --all-datastores table

add_fix() {
    FIX_COMMANDS="${FIX_COMMANDS}$1\n"
}

pass() {
    DEV_PASS=$((DEV_PASS + 1))
    GLOBAL_PASS=$((GLOBAL_PASS + 1))
    printf "${GREEN}[PASS]${NC} %s\n" "$1"
}

warn() {
    DEV_WARN=$((DEV_WARN + 1))
    GLOBAL_WARN=$((GLOBAL_WARN + 1))
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

fail() {
    DEV_FAIL=$((DEV_FAIL + 1))
    GLOBAL_FAIL=$((GLOBAL_FAIL + 1))
    printf "${RED}[FAIL]${NC} %s\n" "$1"
}

info() {
    printf "${CYAN}[INFO]${NC} %s\n" "$1"
}

detail() {
    printf "       %s\n" "$1"
}

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Validate ESXi VAAI/XCOPY readiness for a datastore.

Options:
  --host <host>          ESXi host IP or hostname (or VMWARE_HOST env var)
  --user <user>          ESXi username (or VMWARE_USER env var, default: root)
  --password <password>  ESXi password (or VMWARE_PASSWORD env var)
  --datastore <name>     Datastore name to validate
  --vmdk <path>          VMDK path (alternative to --datastore)
  --all-datastores       Validate all VMFS datastores on the host
  -h, --help             Show this help

Examples:
  $0 --host 10.0.0.1 --user root --password secret --datastore MyDS
  $0 --host 10.0.0.1 --all-datastores
  $0 --datastore MyDS   # uses VMWARE_HOST/USER/PASSWORD env vars
  make validate-xcopy DATASTORE=MyDS
EOF
    exit 0
}

# --- Argument parsing ---
HOST="${VMWARE_HOST:-}"
USER="${VMWARE_USER:-root}"
PASSWORD="${VMWARE_PASSWORD:-}"
DATASTORE=""
VMDK=""
ALL_DATASTORES=false

while [ $# -gt 0 ]; do
    case "$1" in
        --host)           HOST="$2"; shift 2 ;;
        --user)           USER="$2"; shift 2 ;;
        --password)       PASSWORD="$2"; shift 2 ;;
        --datastore)      DATASTORE="$2"; shift 2 ;;
        --vmdk)           VMDK="$2"; shift 2 ;;
        --all-datastores) ALL_DATASTORES=true; shift ;;
        -h|--help)        usage ;;
        *)                echo "Unknown option: $1"; usage ;;
    esac
done

if [ -z "${HOST}" ]; then
    echo "Error: ESXi host is required (--host or VMWARE_HOST env var)"
    exit 1
fi

if [ -z "${PASSWORD}" ]; then
    echo "Error: ESXi password is required (--password or VMWARE_PASSWORD env var)"
    exit 1
fi

if [ "${ALL_DATASTORES}" = false ] && [ -z "${DATASTORE}" ] && [ -z "${VMDK}" ]; then
    echo "Error: --datastore, --vmdk, or --all-datastores is required"
    exit 1
fi

if [ -n "${VMDK}" ] && [ -z "${DATASTORE}" ]; then
    DATASTORE=$(echo "${VMDK}" | sed -n 's|^/vmfs/volumes/\([^/]*\)/.*|\1|p')
    if [ -z "${DATASTORE}" ]; then
        echo "Error: Could not extract datastore name from VMDK path: ${VMDK}"
        exit 1
    fi
fi

if ! command -v sshpass >/dev/null 2>&1; then
    echo "Error: sshpass is required but not installed."
    echo "Install with: dnf install sshpass  (or apt-get install sshpass)"
    exit 1
fi

# --- SSH helper ---
ssh_cmd() {
    SSHPASS="${PASSWORD}" timeout 60 sshpass -e ssh -n \
        -o StrictHostKeyChecking=no \
        -o LogLevel=ERROR \
        -o ConnectTimeout=10 \
        "${USER}@${HOST}" "$1"
}

# --- Collect global ESXi data (one SSH call) ---
collect_esxi_data() {
    local script
    # read -r -d '' always exits 1 at EOF — || true is required
    read -r -d '' script <<'ESXI_SCRIPT' || true
echo "===SECTION:HW_MOVE==="
esxcfg-advcfg -g /DataMover/HardwareAcceleratedMove 2>&1

echo "===SECTION:HW_INIT==="
esxcfg-advcfg -g /DataMover/HardwareAcceleratedInit 2>&1

echo "===SECTION:MAX_XFER==="
esxcfg-advcfg -g /DataMover/MaxHWTransferSize 2>&1

echo "===SECTION:VMFS_EXTENTS==="
esxcli storage vmfs extent list 2>&1

echo "===SECTION:CLAIMRULES==="
esxcli storage core claimrule list --claimrule-class=VAAI 2>&1

echo "===SECTION:VMKNICS==="
esxcfg-vmknic -l 2>&1

echo "===SECTION:ISCSI_TARGETS==="
esxcli iscsi adapter target portal list 2>&1 || true

echo "===SECTION:END==="
ESXI_SCRIPT

    ssh_cmd "${script}"
}

# --- Collect device-specific data (one SSH call per device) ---
collect_device_data() {
    local naa_id="$1"
    local script
    # read -r -d '' always exits 1 at EOF — || true is required
    read -r -d '' script <<ESXI_SCRIPT || true
echo "===SECTION:VAAI_STATUS==="
esxcli storage core device vaai status get -d ${naa_id} 2>&1

echo "===SECTION:NMP_DEVICE==="
esxcli storage nmp device list -d ${naa_id} 2>&1

echo "===SECTION:VSISH_STATS==="
vsish -r -e cat /storage/scsifw/devices/${naa_id}/stats 2>&1 | grep -iE 'clone|xcopy' || true

echo "===SECTION:PATHS==="
esxcli storage core path list -d ${naa_id} 2>&1

echo "===SECTION:END==="
ESXI_SCRIPT

    ssh_cmd "${script}"
}

# --- Run vmkping for selected (vmknic, target_ip) pairs ---
run_vmkping() {
    local vmknic="$1"
    local target_ip="$2"
    local script
    # read -r -d '' always exits 1 at EOF — || true is required
    read -r -d '' script <<ESXI_SCRIPT || true
echo "===PING_SMALL==="
vmkping -4 -c 1 -s 64 -v ${target_ip} -I ${vmknic} 2>&1 || true
echo "===PING_JUMBO==="
vmkping -4 -c 3 -d -s 8972 -v ${target_ip} -I ${vmknic} 2>&1 || true
echo "===PING_END==="
ESXI_SCRIPT

    ssh_cmd "${script}"
}

extract_section() {
    local data="$1"
    local section="$2"
    echo "${data}" | sed -n "/===SECTION:${section}===/,/===SECTION:/{ /===SECTION:/d; p; }"
}

# --- Helper: extract integer value from an advcfg section ---
advcfg_val() {
    echo "${1}" | grep -oE '[0-9]+$' | tail -1 || true
}

# --- Check global host-wide settings (called once) ---
check_global_settings() {
    local all_data="$1"

    printf "\n${BOLD}--- Global ESXi Settings ---${NC}\n"

    local val
    val=$(advcfg_val "$(extract_section "${all_data}" "HW_MOVE")")
    if [ "${val}" = "1" ]; then
        pass "HardwareAcceleratedMove = 1"
    else
        fail "HardwareAcceleratedMove = ${val:-unknown} (must be 1)"
        detail "Fix: esxcfg-advcfg -s 1 /DataMover/HardwareAcceleratedMove"
        add_fix "esxcfg-advcfg -s 1 /DataMover/HardwareAcceleratedMove"
    fi

    val=$(advcfg_val "$(extract_section "${all_data}" "HW_INIT")")
    if [ "${val}" = "1" ]; then
        pass "HardwareAcceleratedInit = 1"
    else
        fail "HardwareAcceleratedInit = ${val:-unknown} (must be 1)"
        detail "Fix: esxcfg-advcfg -s 1 /DataMover/HardwareAcceleratedInit"
        add_fix "esxcfg-advcfg -s 1 /DataMover/HardwareAcceleratedInit"
    fi

    local max_xfer_val
    max_xfer_val=$(advcfg_val "$(extract_section "${all_data}" "MAX_XFER")")
    if [ -z "${max_xfer_val}" ]; then
        warn "MaxHWTransferSize = unknown (could not read value)"
    elif [ "${max_xfer_val}" -ge 16384 ]; then
        pass "MaxHWTransferSize = ${max_xfer_val}"
    elif [ "${max_xfer_val}" -ge 4096 ]; then
        warn "MaxHWTransferSize = ${max_xfer_val} (VMware default; xcopy works but may cause overhead at high I/O concurrency — check storage provider recommendation for optimal performance)"
        detail "Fix: esxcfg-advcfg -s 16384 /DataMover/MaxHWTransferSize"
        add_fix "esxcfg-advcfg -s 16384 /DataMover/MaxHWTransferSize"
    else
        warn "MaxHWTransferSize = ${max_xfer_val} (below VMware default of 4096; xcopy still works but with elevated SCSI command volume — check storage provider recommendation for optimal performance)"
        detail "Fix: esxcfg-advcfg -s 16384 /DataMover/MaxHWTransferSize"
        add_fix "esxcfg-advcfg -s 16384 /DataMover/MaxHWTransferSize"
    fi
}

# --- Check: VAAI Plugin Name + Clone Status ---
check_vaai_status() {
    local vaai_output="$1"
    local vaai_plugin clone_status

    vaai_plugin=$(echo "${vaai_output}" | grep -i "VAAI Plugin Name" | sed 's/.*: *//' | tr -d '\r' || true)

    if [ -n "${vaai_plugin}" ] && [ "${vaai_plugin}" != " " ]; then
        pass "VAAI Plugin Name = ${vaai_plugin}"
    else
        fail "VAAI Plugin Name is empty -- no VAAI plugin bound to device"
        detail "This is often caused by a missing VAAI claim rule for the storage vendor (see below)."
    fi

    clone_status=$(echo "${vaai_output}" | grep -i "Clone Status" | grep -vi "Ex Clone" | head -1 | sed 's/.*: *//' | tr -d '\r' | tr '[:upper:]' '[:lower:]' || true)

    if [ "${clone_status}" = "supported" ]; then
        pass "Clone Status = supported"
    else
        fail "Clone Status = ${clone_status:-unknown} (must be 'supported')"
        detail "The storage array may not support VAAI XCOPY, or the device is not properly configured."
    fi
}

# --- Check: VAAI claim rule for device vendor ---
check_claim_rule() {
    local claimrule_output="$1"
    local vendor_name="$2"
    local model_name="$3"
    local matching_rule rule_plugin

    if [ -n "${vendor_name}" ] && [ "${vendor_name}" != "unknown" ]; then
        matching_rule=$(echo "${claimrule_output}" | grep -i "vendor=${vendor_name}" | head -1 || true)

        if [ -n "${matching_rule}" ]; then
            rule_plugin=$(echo "${matching_rule}" | awk '{print $5}')
            pass "VAAI claim rule exists for vendor=${vendor_name} (plugin: ${rule_plugin})"
        else
            fail "No VAAI claim rule found for vendor=${vendor_name}"
            detail "Without a claim rule, ESXi cannot bind a VAAI plugin to this vendor's devices."
            if [ -n "${model_name}" ]; then
                detail "Suggested fix:"
                detail "  esxcli storage core claimrule add -r 911 --claimrule-class=VAAI \\"
                detail "    -t vendor -V ${vendor_name} -M '${model_name}' -P VMW_VAAIP_T10 -a -s -m 16384"
                detail "  esxcli storage core claimrule load --claimrule-class=VAAI"
                detail "  esxcli storage core claimrule run --claimrule-class=VAAI"
                add_fix "esxcli storage core claimrule add -r 911 --claimrule-class=VAAI -t vendor -V ${vendor_name} -M '${model_name}' -P VMW_VAAIP_T10 -a -s -m 16384"
                add_fix "esxcli storage core claimrule load --claimrule-class=VAAI"
                add_fix "esxcli storage core claimrule run --claimrule-class=VAAI"
            fi
        fi
    else
        warn "Could not determine device vendor -- skipping claim rule check"
    fi
}

# --- Check: FC/iSCSI path health; sets global IS_ISCSI as side-effect ---
check_path_health() {
    local paths_output="$1"
    local active_paths standby_paths dead_paths total_paths dead_names

    active_paths=$(echo "${paths_output}" | grep -c "State: active" || true)
    standby_paths=$(echo "${paths_output}" | grep -c "State: standby" || true)
    dead_paths=$(echo "${paths_output}" | grep -cE "State: dead|State: error" || true)
    total_paths=$((active_paths + standby_paths + dead_paths))

    if [ "${total_paths}" -gt 0 ]; then
        if [ "${active_paths}" -eq 0 ]; then
            fail "Path health: ${total_paths} paths (${active_paths} active, ${standby_paths} standby, ${dead_paths} dead) — device unreachable"
        elif [ "${dead_paths}" -gt 0 ]; then
            dead_names=$(echo "${paths_output}" | awk '
                /^   Runtime Name:/ { name=$NF }
                /State: dead|State: error/ { print name }
            ' | tr '\n' ' ')
            warn "Path health: ${total_paths} paths (${active_paths} active, ${standby_paths} standby, ${dead_paths} dead) — multipath degraded: ${dead_names}"
        else
            pass "Path health: ${total_paths} paths (${active_paths} active, ${standby_paths} standby, 0 dead)"
        fi
    else
        warn "Path health: no path data available"
    fi

    # Set global IS_ISCSI (used by caller to decide whether to run jumbo frame check)
    IS_ISCSI=$(echo "${paths_output}" | grep "Adapter Transport Details:" | grep "iqn\." | head -1 || true)
}

# --- Check: vsish clone operation counters ---
check_clone_stats() {
    local vsish_output="$1"
    local clone_read clone_write failed_clone

    if [ -n "${vsish_output}" ]; then
        clone_read=$(echo "${vsish_output}" | grep -i "cloneReadOps" | awk -F: '{print $2}' | sed 's/[[:space:]]//g' || true)
        clone_write=$(echo "${vsish_output}" | grep -i "cloneWriteOps" | awk -F: '{print $2}' | sed 's/[[:space:]]//g' || true)
        failed_clone=$(echo "${vsish_output}" | grep -i "failedCloneOps" | awk -F: '{print $2}' | sed 's/[[:space:]]//g' || true)

        if [ -n "${clone_write}" ] && [ "${clone_write}" != "0x0000000000000000" ]; then
            info "Clone stats: cloneReadOps=${clone_read:-0} cloneWriteOps=${clone_write} (xcopy operations recorded)"
        else
            info "Clone stats: cloneReadOps=${clone_read:-0x0} cloneWriteOps=${clone_write:-0x0} (no xcopy operations recorded)"
        fi

        if [ -n "${failed_clone}" ] && [ "${failed_clone}" != "0x0000000000000000" ]; then
            warn "failedCloneOps=${failed_clone} -- some xcopy operations failed"
        fi
    else
        warn "Could not read vsish clone stats (vsish may not be available)"
    fi
}

# --- Check a single device (orchestrator) ---
check_device() {
    local naa_id="$1"
    local ds_name="$2"
    local all_data="$3"
    local device_data="$4"

    # Reset per-device counters and transport detection result
    # IS_ISCSI is a global used as a return value — reset here so callers
    # always see the result from the most recent check_device call
    DEV_PASS=0
    DEV_WARN=0
    DEV_FAIL=0
    IS_ISCSI=""
    FIX_COMMANDS=""

    # --- Device info ---
    local nmp_output device_vendor device_satp vendor_name model_name
    nmp_output=$(extract_section "${device_data}" "NMP_DEVICE")
    device_vendor=$(echo "${nmp_output}" | grep -i "Device Display Name" | sed 's/.*: *//' | tr -d '\r' || true)
    device_satp=$(echo "${nmp_output}" | grep -i "Storage Array Type:" | head -1 | sed 's/.*: *//' | tr -d '\r' || true)
    vendor_name=""
    model_name=""

    if [ -n "${device_vendor}" ]; then
        model_name=$(echo "${device_vendor}" | sed -n 's/^[^ ]* \(.*\) ([^)]*)/\1/p')
        vendor_name=$(echo "${device_vendor}" | awk '{print $1}')
    fi

    printf "\n${BOLD}--- Device-Specific Checks ---${NC}\n"
    printf "Device:    %s\n" "${naa_id}"
    printf "Vendor:    %s\n" "${vendor_name:-unknown}"
    [ -n "${model_name}" ] && printf "Model:     %s\n" "${model_name}"
    printf "SATP:      %s\n" "${device_satp:-unknown}"
    echo ""

    check_vaai_status "$(extract_section "${device_data}" "VAAI_STATUS")"
    check_claim_rule "$(extract_section "${all_data}" "CLAIMRULES")" "${vendor_name}" "${model_name}"
    check_path_health "$(extract_section "${device_data}" "PATHS")"
    check_clone_stats "$(extract_section "${device_data}" "VSISH_STATS")"
}

# --- Print per-device summary ---
print_device_summary() {
    local ds_name="$1"
    local naa_id="$2"

    echo ""
    if [ "${DEV_FAIL}" -eq 0 ] && [ "${DEV_WARN}" -eq 0 ]; then
        printf "${GREEN}${BOLD}=== Summary [%s]: %d PASS, %d WARN, %d FAIL — XCOPY should work ===${NC}\n" \
            "${ds_name}" "${DEV_PASS}" "${DEV_WARN}" "${DEV_FAIL}"
    elif [ "${DEV_FAIL}" -eq 0 ]; then
        printf "${YELLOW}${BOLD}=== Summary [%s]: %d PASS, %d WARN, %d FAIL — XCOPY may work but check warnings ===${NC}\n" \
            "${ds_name}" "${DEV_PASS}" "${DEV_WARN}" "${DEV_FAIL}"
    else
        printf "${RED}${BOLD}=== Summary [%s]: %d PASS, %d WARN, %d FAIL — XCOPY configuration issues ===${NC}\n" \
            "${ds_name}" "${DEV_PASS}" "${DEV_WARN}" "${DEV_FAIL}"
    fi

    if [ -n "${FIX_COMMANDS}" ]; then
        echo ""
        printf "${BOLD}--- How to fix [%s] (run on ESXi host ${HOST}) ---${NC}\n" "${ds_name}"
        printf '%b' "${FIX_COMMANDS}" | while IFS= read -r cmd; do
            [ -n "${cmd}" ] && printf "  %s\n" "${cmd}"
        done
        printf "${YELLOW}Note: These changes are runtime-only unless the claim rule uses -a (auto-add to config).${NC}\n"
    fi
}

# --- iSCSI jumbo frames interactive check ---
check_jumbo_frames() {
    local all_data="$1"
    local vmknics_output vmknic_list targets_output target_list
    local test_another i vmknic_arr ip_arr entry
    local iface ip mtu jumbo_label
    local vmknic_choice selected_vmknic
    local tip tiqn ip_choices choice target_ip
    local ping_output small_output jumbo_output small_received jumbo_received

    echo ""
    printf "${BOLD}=== iSCSI Jumbo Frames Test ===${NC}\n"

    # Parse all vmknics with their IPs and MTUs (IPv4 lines only)
    vmknics_output=$(extract_section "${all_data}" "VMKNICS")

    # Build array of "vmknic ip mtu" tuples from IPv4 lines
    vmknic_list=""
    while IFS= read -r line; do
        # Interface lines start with vmkN (not spaces)
        if echo "${line}" | grep -qE '^vmk[0-9]'; then
            iface=$(echo "${line}" | awk '{print $1}')
            ip=$(echo "${line}" | awk '{print $4}')
            mtu=$(echo "${line}" | awk '{print $8}')
            # Only IPv4 lines (IP field looks like x.x.x.x)
            if echo "${ip}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
                vmknic_list="${vmknic_list}${iface} ${ip} ${mtu}\n"
            fi
        fi
    done <<EOF
$(echo "${vmknics_output}")
EOF

    if [ -z "${vmknic_list}" ]; then
        info "No VMkernel NICs found — skipping jumbo frames test"
        return
    fi

    # Parse iSCSI target IPs
    targets_output=$(extract_section "${all_data}" "ISCSI_TARGETS")
    # Skip header lines (Adapter, ---)
    target_list=$(echo "${targets_output}" | grep -vE "^Adapter|^----|^$" | awk '{print $3, $2}' || true)

    if [ -z "${target_list}" ]; then
        info "No iSCSI target portals found — skipping jumbo frames test"
        return
    fi

    # Interactive loop
    test_another="y"
    while [ "${test_another}" = "y" ]; do

        # Show vmknic menu
        echo ""
        printf "Detected VMkernel NICs:\n"
        i=1
        vmknic_arr=""
        while IFS= read -r entry; do
            [ -z "${entry}" ] && continue
            iface=$(echo "${entry}" | awk '{print $1}')
            ip=$(echo "${entry}" | awk '{print $2}')
            mtu=$(echo "${entry}" | awk '{print $3}')
            jumbo_label=""
            if echo "${mtu}" | grep -qE '^[0-9]+$' && [ "${mtu}" -ge 9000 ]; then
                jumbo_label="  ${YELLOW}← jumbo frames configured${NC}"
            fi
            printf "  %d. %-6s (%-15s MTU %s)%b\n" "${i}" "${iface}" "${ip}" "${mtu}" "${jumbo_label}"
            vmknic_arr="${vmknic_arr}${iface}\n"
            i=$((i + 1))
        done <<EOF
$(printf '%b' "${vmknic_list}")
EOF

        printf "\nChoose interface to test (enter number): "
        read -r vmknic_choice || break   # EOF (non-interactive) → exit loop
        [ -z "${vmknic_choice}" ] && break
        if ! echo "${vmknic_choice}" | grep -qE '^[0-9]+$'; then
            printf "${RED}Invalid choice '%s'. Enter a number from the list above.${NC}\n" "${vmknic_choice}"
            continue
        fi
        selected_vmknic=$(printf '%b' "${vmknic_list}" | awk -v n="${vmknic_choice}" 'NR==n{print $1}')

        if [ -z "${selected_vmknic}" ]; then
            printf "${RED}Invalid choice '%s'. Enter a number from the list above.${NC}\n" "${vmknic_choice}"
            continue
        fi

        if ! echo "${selected_vmknic}" | grep -qE '^vmk[0-9]+$'; then
            printf "${RED}Unexpected vmknic name '%s' — skipping.${NC}\n" "${selected_vmknic}"
            continue
        fi

        # Show target IP menu
        echo ""
        printf "Detected iSCSI target IPs:\n"
        printf "  0. All\n"
        i=1
        ip_arr=""
        while IFS= read -r entry; do
            [ -z "${entry}" ] && continue
            tip=$(echo "${entry}" | awk '{print $1}')
            tiqn=$(echo "${entry}" | awk '{print $2}')
            printf "  %d. %-15s (%s)\n" "${i}" "${tip}" "${tiqn}"
            ip_arr="${ip_arr}${tip}\n"
            i=$((i + 1))
        done <<EOF
$(echo "${target_list}")
EOF
        local ip_count=$((i - 1))

        printf "\nChoose target IPs to test (0 for all, or comma-delimited, e.g. 1,3): "
        read -r ip_choices || true

        # Expand "0" / "all" to every index
        if echo "${ip_choices}" | grep -qE '^(0|all)$'; then
            ip_choices=$(seq 1 "${ip_count}" | tr '\n' ',')
        fi

        # Run vmkping for each selected IP
        echo ""
        for choice in $(echo "${ip_choices}" | tr ',' ' '); do
            if ! echo "${choice}" | grep -qE '^[0-9]+$'; then continue; fi
            target_ip=$(printf '%b' "${ip_arr}" | awk -v n="${choice}" 'NR==n{print $1}')
            [ -z "${target_ip}" ] && continue
            if ! echo "${target_ip}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$'; then
                printf "${YELLOW}[WARN]${NC} Skipping invalid IP: %s\n" "${target_ip}"
                continue
            fi

            ping_output=$(run_vmkping "${selected_vmknic}" "${target_ip}")

            small_output=$(echo "${ping_output}" | sed -n '/===PING_SMALL===/,/===PING_JUMBO===/{ /===PING/d; p; }')
            jumbo_output=$(echo "${ping_output}" | sed -n '/===PING_JUMBO===/,/===PING_END===/{ /===PING/d; p; }')

            small_received=$(echo "${small_output}" | grep -oE '[0-9]+ packets received' | grep -oE '^[0-9]+' || true)
            jumbo_received=$(echo "${jumbo_output}" | grep -oE '[0-9]+ packets received' | grep -oE '^[0-9]+' || true)

            if [ "${small_received:-0}" = "0" ]; then
                warn "${selected_vmknic} → ${target_ip}: target unreachable (routing issue, not MTU)"
            elif [ "${jumbo_received:-0}" = "0" ]; then
                fail "${selected_vmknic} → ${target_ip}: jumbo frames BROKEN — check MTU on switch and storage iSCSI ports"
                add_fix "# Fix: ensure MTU=9000 on ESXi vmknic ${selected_vmknic}, network switches, and storage array iSCSI ports targeting ${target_ip}"
            else
                pass "${selected_vmknic} → ${target_ip}: jumbo frames OK (${jumbo_received}/3 packets)"
            fi
        done

        printf "\nTest another interface? (y/n): "
        read -r test_another || break
        test_another=$(echo "${test_another}" | tr '[:upper:]' '[:lower:]')
    done
}

# --- Main ---
echo ""
printf "${BOLD}=== XCOPY Readiness Validation ===${NC}\n"
printf "Host: %s\n" "${HOST}"
echo ""

printf "${YELLOW}Note: SSH host key verification is disabled — use on trusted networks only.${NC}\n"
printf "Connecting to ESXi host...\n"
ALL_DATA=$(collect_esxi_data)

check_global_settings "${ALL_DATA}"

vmfs_output=$(extract_section "${ALL_DATA}" "VMFS_EXTENTS")

if [ "${ALL_DATASTORES}" = true ]; then
    # --- All-datastores mode ---
    ds_list=$(echo "${vmfs_output}" | grep -vE "^$|^-|Volume Name|^OSDATA-" | awk '{print $1}' | sort -u)

    if [ -z "${ds_list}" ]; then
        echo ""
        printf "${RED}No VMFS datastores found on ESXi host ${HOST}.${NC}\n"
        exit 1
    fi

    local_ds_summary=""
    iscsi_detected_global=false

    for ds in ${ds_list}; do
        echo ""
        printf "${BOLD}╔══ Datastore: %s ══╗${NC}\n" "${ds}"

        naa_id=$(echo "${vmfs_output}" | awk -v d="${ds}" '$1 == d {print $4}' | head -1)
        if [ -z "${naa_id}" ]; then
            printf "${YELLOW}[SKIP]${NC} Could not resolve NAA for datastore %s\n" "${ds}"
            continue
        fi

        if ! echo "${naa_id}" | grep -qE '^naa\.[0-9a-f]{16,64}$'; then
            printf "${CYAN}[INFO]${NC} Skipping %s — local disk (%s), no VAAI support\n" "${ds}" "${naa_id}"
            continue
        fi

        DEVICE_DATA=$(collect_device_data "${naa_id}")
        check_device "${naa_id}" "${ds}" "${ALL_DATA}" "${DEVICE_DATA}"

        # Build summary row
        if [ "${DEV_FAIL}" -gt 0 ]; then
            status="${RED}XCOPY ISSUES${NC}"
        elif [ "${DEV_WARN}" -gt 0 ]; then
            status="${YELLOW}OK (warnings)${NC}"
        else
            status="${GREEN}OK${NC}"
        fi
        local_ds_summary="${local_ds_summary}  $(printf '%-30s' "${ds}") $(printf '%-16s' "(${naa_id:0:12}...)") ${DEV_PASS} PASS  ${DEV_WARN} WARN  ${DEV_FAIL} FAIL  ${status}\n"

        print_device_summary "${ds}" "${naa_id}"

        if [ -n "${IS_ISCSI}" ]; then
            iscsi_detected_global=true
        fi
    done

    # All-datastores summary table
    echo ""
    printf "${BOLD}=== All-Datastores Summary ===${NC}\n"
    printf '%b' "${local_ds_summary}" | while IFS= read -r row; do
        [ -n "${row}" ] && printf "%b\n" "${row}"
    done

    # Offer jumbo frames test if iSCSI detected (only in interactive terminal)
    if [ "${iscsi_detected_global}" = true ] && [ -t 0 ]; then
        echo ""
        printf "iSCSI datastores detected. Run jumbo frames test? (y/n): "
        run_jumbo=""
        read -r run_jumbo || true
        run_jumbo=$(echo "${run_jumbo}" | tr '[:upper:]' '[:lower:]')
        if [ "${run_jumbo}" = "y" ]; then
            check_jumbo_frames "${ALL_DATA}"
        fi
    fi

else
    # --- Single datastore mode ---
    naa_id=$(echo "${vmfs_output}" | awk -v ds="${DATASTORE}" '$1 == ds {print $4}' | head -1)

    if [ -z "${naa_id}" ]; then
        echo ""
        printf "${RED}Datastore '${DATASTORE}' was not found on ESXi host ${HOST}.${NC}\n"
        printf "Check the datastore name or choose one of the following:\n\n"
        echo "${vmfs_output}" | grep -vE "^$|^-|Volume Name" | awk '{print "  " $1}'
        echo ""
        exit 1
    fi

    if ! echo "${naa_id}" | grep -qE '^naa\.[0-9a-f]{16,64}$'; then
        echo ""
        printf "${RED}Device '%s' is not a supported NAA device — cannot run VAAI checks.${NC}\n" "${naa_id}"
        exit 1
    fi

    DEVICE_DATA=$(collect_device_data "${naa_id}")
    check_device "${naa_id}" "${DATASTORE}" "${ALL_DATA}" "${DEVICE_DATA}"

    if [ -n "${IS_ISCSI}" ]; then
        check_jumbo_frames "${ALL_DATA}"
    fi

    print_device_summary "${DATASTORE}" "${naa_id}"
fi

[ "${GLOBAL_FAIL}" -gt 125 ] && GLOBAL_FAIL=125
exit ${GLOBAL_FAIL}
