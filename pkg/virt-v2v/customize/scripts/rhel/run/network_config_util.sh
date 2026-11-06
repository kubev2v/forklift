#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
V2V_POD_NETWORK_IPV6_MACS_FILE="${V2V_POD_NETWORK_IPV6_MACS_FILE:-/tmp/podNetworkIPv6MACs}"
KVIRT_POD_IPV6_GW="fd10:0:2::1"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_SCRIPTS_DIR_SUSE="${NETWORK_SCRIPTS_DIR_SUSE:-/etc/sysconfig/network}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
NM_LEASES_DIR="${NM_LEASES_DIR:-/var/lib/NetworkManager}"
DHCLIENT_LEASES_DIR="${DHCLIENT_LEASES_DIR:-/var/lib/dhclient}"
NETWORK_INTERFACES_DIR="${NETWORK_INTERFACES_DIR:-/etc/network/interfaces}"
IFQUERY_CMD="${IFQUERY_CMD:-ifquery}"
SYSTEMD_NETWORK_DIR="${SYSTEMD_NETWORK_DIR:-/run/systemd/network}"
WICKED_DIR="${WICKED_DIR:-/var/lib/wicked}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"
SYSTEMD_LINK_DIR="${SYSTEMD_LINK_DIR:-/etc/systemd/network}"
NETPLAN_DIR="${NETPLAN_DIR:-/}"

# Dump debug strings into a new file descriptor and redirect it to stdout.
exec 3>&1
log() {
    echo "$@" >&3
}

normalize_mac() {
    echo "$1" | tr 'A-F' 'a-f' | tr -d '[:space:]'
}

mac_in_pod_list() {
    local want
    want=$(normalize_mac "$1")
    [ -z "$want" ] && return 1
    while read -r line; do
        line=$(normalize_mac "$line")
        [ -z "$line" ] && continue
        [ "$line" = "$want" ] && return 0
    done < "$V2V_POD_NETWORK_IPV6_MACS_FILE"
    return 1
}

nmconnection_mac() {
    local f="$1" mac
    mac=$(sed -n '/^\[ethernet\]/,/^\[/{ /^\(mac-address\|cloned-mac-address\)=/p; }' "$f" | head -1 | cut -d= -f2-)
    if [ -z "$mac" ]; then
        mac=$(grep -m1 -iE '^(mac-address|cloned-mac-address)=' "$f" | cut -d= -f2-)
    fi
    normalize_mac "$mac"
}

ifcfg_mac() {
    local f="$1" mac
    mac=$(grep -m1 -iE '^HWADDR=' "$f" | cut -d= -f2-)
    # Strip surrounding quotes (e.g. HWADDR="00:50:56:97:33:D8").
    mac=$(echo "$mac" | tr -d '"'"'")
    normalize_mac "$mac"
}

set_nm_keyfile_kv() {
    local f="$1" key="$2" val="$3"
    if sed -n "/^\[ipv6\]/,/^\[/{ /^${key}=/p; }" "$f" | grep -q .; then
        sed -i "/^\[ipv6\]/,/^\[/ s|^${key}=.*|${key}=${val}|" "$f"
    else
        sed -i "/^\[ipv6\]/a ${key}=${val}" "$f"
    fi
}

set_connection_keyfile_kv() {
    local f="$1" key="$2" val="$3"
    if sed -n "/^\[connection\]/,/^\[/{ /^${key}=/p; }" "$f" | grep -q .; then
        sed -i "/^\[connection\]/,/^\[/ s/^${key}=.*/${key}=${val}/" "$f"
    else
        sed -i "/^\[connection\]/a ${key}=${val}" "$f"
    fi
}

set_ipv4_keyfile_kv() {
    local f="$1" key="$2" val="$3"
    if sed -n "/^\[ipv4\]/,/^\[/{ /^${key}=/p; }" "$f" | grep -q .; then
        sed -i "/^\[ipv4\]/,/^\[/ s/^${key}=.*/${key}=${val}/" "$f"
    else
        sed -i "/^\[ipv4\]/a ${key}=${val}" "$f"
    fi
}

udev_iface_name_for_mac() {
    local want="$1" line mac name
    [ ! -f "$UDEV_RULES_FILE" ] && return 1
    while read -r line; do
        mac=$(echo "$line" | sed -n 's/.*ATTR{address}=="\([^"]*\)".*/\1/p')
        name=$(echo "$line" | sed -n 's/.*NAME="\([^"]*\)".*/\1/p')
        [ -z "$mac" ] || [ -z "$name" ] && continue
        [ "$(normalize_mac "$mac")" = "$(normalize_mac "$want")" ] && { echo "$name"; return 0; }
    done < "$UDEV_RULES_FILE"
    return 1
}

patch_nm_keyfile_pod() {
    local NM_FILE="$1" mac="$2" udev_name=""
    log "Fixing Pod network profile $NM_FILE"

    if udev_name=$(udev_iface_name_for_mac "$mac"); then
        set_connection_keyfile_kv "$NM_FILE" interface-name "$udev_name"
        log "Pod profile $NM_FILE: interface-name=$udev_name (from udev)"
    else
        # Stale source names (e.g. ens192) prevent binding after udev renames to eth0.
        sed -i '/^\[connection\]/,/^\[/{ /^interface-name=/d; }' "$NM_FILE"
    fi
    set_connection_keyfile_kv "$NM_FILE" autoconnect true
    # Source VMs often bridge the NIC (e.g. to virbr0), Pod network needs a plain ethernet profile.
    sed -i '/^\[connection\]/,/^\[/{ /^\(master\|slave-type\|port-type\)=/d; }' "$NM_FILE"

    if ! grep -q '^\[ipv4\]' "$NM_FILE"; then
        printf '\n[ipv4]\nmethod=auto\nmay-fail=true\n' >> "$NM_FILE"
    else
        sed -i '/^\[ipv4\]/,/^\[/ s/^method=.*/method=auto/' "$NM_FILE"
        set_ipv4_keyfile_kv "$NM_FILE" may-fail true
    fi

    if ! grep -q '^\[ipv6\]' "$NM_FILE"; then
        printf '\n[ipv6]\nmethod=auto\nmay-fail=true\ngateway=%s\nroute1=::/0,%s\n' "$KVIRT_POD_IPV6_GW" "$KVIRT_POD_IPV6_GW" >> "$NM_FILE"
    else
        sed -i '/^\[ipv6\]/,/^\[/ s/^method=.*/method=auto/' "$NM_FILE"
        set_nm_keyfile_kv "$NM_FILE" may-fail true
        set_nm_keyfile_kv "$NM_FILE" gateway "$KVIRT_POD_IPV6_GW"
        set_nm_keyfile_kv "$NM_FILE" route1 "::/0,${KVIRT_POD_IPV6_GW}"
    fi
    chmod 600 "$NM_FILE" 2>/dev/null || true
}

patch_ifcfg_pod() {
    local IFCFG="$1" mac="$2" udev_name=""
    log "Fixing Pod network ifcfg $IFCFG"
    if udev_name=$(udev_iface_name_for_mac "$mac"); then
        if grep -q '^DEVICE=' "$IFCFG"; then
            sed -i "s/^DEVICE=.*/DEVICE=${udev_name}/" "$IFCFG"
        else
            echo "DEVICE=${udev_name}" >> "$IFCFG"
        fi
    else
        # Remove stale DEVICE that may not match the destination name.
        # HWADDR is sufficient for NM to bind the profile to the right NIC.
        sed -i '/^DEVICE=/d' "$IFCFG"
    fi
    sed -i '/^BRIDGE=/d; /^MASTER=/d; /^SLAVE=/d' "$IFCFG"

    # Ensure IPv4 DHCP failure is non-fatal (IPv6-only clusters have no IPv4).
    if grep -q '^IPV4_FAILURE_FATAL=' "$IFCFG"; then
        sed -i 's/^IPV4_FAILURE_FATAL=.*/IPV4_FAILURE_FATAL=no/' "$IFCFG"
    else
        echo "IPV4_FAILURE_FATAL=no" >> "$IFCFG"
    fi

    # Enable IPv6 with autoconf and non-fatal.
    grep -q '^IPV6INIT=' "$IFCFG" || echo "IPV6INIT=yes" >> "$IFCFG"
    grep -q '^IPV6_AUTOCONF=' "$IFCFG" || echo "IPV6_AUTOCONF=yes" >> "$IFCFG"
    if grep -q '^IPV6_FAILURE_FATAL=' "$IFCFG"; then
        sed -i 's/^IPV6_FAILURE_FATAL=.*/IPV6_FAILURE_FATAL=no/' "$IFCFG"
    else
        echo "IPV6_FAILURE_FATAL=no" >> "$IFCFG"
    fi
    if grep -q '^IPV6_DEFAULTGW=' "$IFCFG"; then
        sed -i "s/^IPV6_DEFAULTGW=.*/IPV6_DEFAULTGW=${KVIRT_POD_IPV6_GW}/" "$IFCFG"
    else
        echo "IPV6_DEFAULTGW=${KVIRT_POD_IPV6_GW}" >> "$IFCFG"
    fi

    # Pin HWADDR so the profile can be matched by MAC (not just device name).
    if ! grep -qi '^HWADDR=' "$IFCFG"; then
        echo "HWADDR=${mac}" >> "$IFCFG"
    fi
}

# Match an ifcfg file to a Pod MAC. Checks HWADDR first, then falls back to
# DEVICE name matching the udev name for that MAC (covers ifcfg-NIC-1 style
# files that have DEVICE=eth0 but no HWADDR).
ifcfg_matches_pod_mac() {
    local IFCFG="$1" want_mac="$2"
    local file_mac
    file_mac=$(ifcfg_mac "$IFCFG")
    if [ -n "$file_mac" ]; then
        [ "$file_mac" = "$want_mac" ] && return 0
        return 1
    fi
    # No HWADDR — check if DEVICE matches the udev name for this MAC.
    local dev udev_name
    dev=$(grep -m1 '^DEVICE=' "$IFCFG" | cut -d= -f2 | tr -d '"'"'" | tr -d '[:space:]')
    [ -z "$dev" ] && return 1
    udev_name=$(udev_iface_name_for_mac "$want_mac") || return 1
    [ "$dev" = "$udev_name" ] && return 0
    return 1
}

# Create a persistent NM config for a Pod MAC when no config file exists on disk.
# On RHEL7/8 guests (ifcfg-rh plugin), create an ifcfg file.
# On RHEL9+ and others (keyfile plugin), create an .nmconnection keyfile.
create_pod_network_config() {
    local mac="$1" udev_name=""

    udev_name=$(udev_iface_name_for_mac "$mac") || true

    # Derive a safe filename from the MAC (colons → dashes).
    local safe_mac
    safe_mac=$(echo "$mac" | tr ':' '-')

    # Prefer ifcfg when the directory exists (RHEL7/8 with ifcfg-rh plugin).
    if [ -d "$NETWORK_SCRIPTS_DIR" ]; then
        local IFCFG="${NETWORK_SCRIPTS_DIR}/ifcfg-pod-${safe_mac}"
        log "Creating Pod network ifcfg $IFCFG for MAC $mac"
        cat > "$IFCFG" <<EOF
TYPE=Ethernet
BOOTPROTO=dhcp
ONBOOT=yes
HWADDR=${mac}
IPV4_FAILURE_FATAL=no
IPV6INIT=yes
IPV6_AUTOCONF=yes
IPV6_FAILURE_FATAL=no
EOF
        echo "IPV6_DEFAULTGW=${KVIRT_POD_IPV6_GW}" >> "$IFCFG"
        # Only pin DEVICE when we know the correct name from udev.
        # A wrong DEVICE (e.g. eth0 when the NIC is ens3) prevents activation.
        if [ -n "$udev_name" ]; then
            echo "DEVICE=${udev_name}" >> "$IFCFG"
        fi
        chmod 644 "$IFCFG"
        return 0
    fi

    # Fall back to keyfile (.nmconnection) for RHEL9+ / systems without ifcfg.
    mkdir -p "$NETWORK_CONNECTIONS_DIR"
    local iface_clause=""
    [ -n "$udev_name" ] && iface_clause="interface-name=${udev_name}"

    local NM_FILE="${NETWORK_CONNECTIONS_DIR}/pod-${safe_mac}.nmconnection"
    log "Creating Pod network keyfile $NM_FILE for MAC $mac"
    cat > "$NM_FILE" <<EOF
[connection]
id=pod-${safe_mac}
type=ethernet
autoconnect=true
${iface_clause}

[ethernet]
mac-address=${mac}

[ipv4]
method=auto
may-fail=true

[ipv6]
method=auto
may-fail=true
EOF
    echo "gateway=${KVIRT_POD_IPV6_GW}" >> "$NM_FILE"
    echo "route1=::/0,${KVIRT_POD_IPV6_GW}" >> "$NM_FILE"
    # Remove blank interface-name line when udev name was not available.
    sed -i '/^$/d' "$NM_FILE"
    chmod 600 "$NM_FILE"
}

# Pod masquerade: static IPv6 gateway + non-fatal IPv6 (keeps DHCP connections up).
# Only runs when /tmp/podNetworkIPv6MACs exists (IPv6-enabled clusters only).
fix_pod_network_ipv6() {
    if [ ! -s "$V2V_POD_NETWORK_IPV6_MACS_FILE" ]; then
        log "No $V2V_POD_NETWORK_IPV6_MACS_FILE; skipping Pod network IPv6 fix."
        return 0
    fi

    # Track which Pod MACs we successfully patched so we know which ones need
    # a new keyfile created from scratch.
    local patched_macs=""

    if [ -d "$NETWORK_CONNECTIONS_DIR" ]; then
        for NM_FILE in "$NETWORK_CONNECTIONS_DIR"/*.nmconnection; do
            [ -f "$NM_FILE" ] || continue
            mac=$(nmconnection_mac "$NM_FILE")
            mac_in_pod_list "$mac" || continue
            patch_nm_keyfile_pod "$NM_FILE" "$mac"
            patched_macs="${patched_macs} $(normalize_mac "$mac")"
        done
    fi

    local SCRIPTS_DIR=""
    [ -d "$NETWORK_SCRIPTS_DIR" ] && SCRIPTS_DIR="$NETWORK_SCRIPTS_DIR"
    [ -d "$NETWORK_SCRIPTS_DIR_SUSE" ] && SCRIPTS_DIR="$NETWORK_SCRIPTS_DIR_SUSE"
    if [ -n "$SCRIPTS_DIR" ]; then
        while read -r pod_mac_line; do
            local want_mac
            want_mac=$(normalize_mac "$pod_mac_line")
            [ -z "$want_mac" ] && continue
            for IFCFG in "$SCRIPTS_DIR"/ifcfg-*; do
                [ -f "$IFCFG" ] || continue
                case "$(basename "$IFCFG")" in ifcfg-lo|*.bak|*.orig|*~) continue ;; esac
                ifcfg_matches_pod_mac "$IFCFG" "$want_mac" || continue
                patch_ifcfg_pod "$IFCFG" "$want_mac"
                patched_macs="${patched_macs} ${want_mac}"
                break
            done
        done < "$V2V_POD_NETWORK_IPV6_MACS_FILE"
    fi

    # For any Pod MAC that had no config file on disk, create a new keyfile.
    while read -r line; do
        line=$(normalize_mac "$line")
        [ -z "$line" ] && continue
        case "$patched_macs" in
            *"$line"*) continue ;;
        esac
        create_pod_network_config "$line"
    done < "$V2V_POD_NETWORK_IPV6_MACS_FILE"
}

# Sanity checks
# -------------

# Check if mapping file does not exist
if [ ! -f "$V2V_MAP_FILE" ]; then
    fix_pod_network_ipv6
    log "File $V2V_MAP_FILE does not exist. Exiting."
    exit 0
fi

# Check if udev rules file exists and is not empty
if [ -f "$UDEV_RULES_FILE" ] && [ -s "$UDEV_RULES_FILE" ]; then
    fix_pod_network_ipv6
    log "File $UDEV_RULES_FILE already exists and is not empty. Exiting."
    exit 0
fi

# Helper functions
# ----------------

# Clean strings in case they have quotes
remove_quotes() {
    echo "$1" | tr -d '"' | tr -d "'" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

# Validate MAC address and IPv4 address and extract them
extract_mac_ip() {
    S_HW=""
    S_IP=""
    if echo "$1" | grep -qE '^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$'; then
        S_HW=$(echo "$1" | sed -nE 's/^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:.*$/\1/p')
        S_IP=$(echo "$1" | sed -nE 's/^.*:ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$/\1/p')
    fi
}

# Network infrastructure reading functions
# ----------------------------------------

get_device_from_ifcfg() {
    local IFCFG="$1"
    local S_HW="$2"

    # Check for DEVICE in the config file
    DEVICE=$(grep '^DEVICE=' "$IFCFG" | cut -d'=' -f2)
    if [ -n "$DEVICE" ]; then
        echo "$DEVICE"
        return
    fi

    # If no DEVICE, check for HWADDR and ensure S_HW is part of HWADDR (case-insensitive)
    HWADDR=$(grep '^HWADDR=' "$IFCFG" | cut -d'=' -f2)
    if echo "$HWADDR" | grep -iq "$S_HW"; then
        # Extract device name from the file name, using last part after splitting by "-"
        echo "$(basename "$IFCFG" | awk -F'-' '{print $NF}')"
        return
    fi

    # Return an empty string if no valid device is found
    echo ""
}

# Create udev rules based on the macToIP mapping + SUSE wicked DHCP leases.
# Wicked stores lease files as XML in /var/lib/wicked/ with filenames like
# lease-<interface>-dhcp-ipv4.xml, where the interface name is the second
# dash-delimited field of the filename.
udev_from_wicked() {
    # Check if the SUSE wicked dir exist
    if [ ! -d "$WICKED_DIR" ]; then
        log "Warning: Directory $WICKED_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read -r line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Find the matching wicked connection file
        WICKED_FILE=$(grep -El "<address>$S_IP</address>" "$WICKED_DIR"/*)
        if [ -z "$WICKED_FILE" ]; then
            log "Info: no wicked file name found for $S_IP."
            continue
        fi

        # Extract the DEVICE (interface name) from the matching file
        DEVICE=$(basename "$WICKED_FILE" | cut -d'-' -f2)
        if [ -z "$DEVICE" ]; then
            log "Info: no interface name found to $S_IP."
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done
}

# Create udev rules based on the macToip mapping + ifcfg network scripts
# Supports both RHEL (/etc/sysconfig/network-scripts/) and SUSE (/etc/sysconfig/network/)
# Automatically detects which path exists and uses it (RHEL path takes precedence)
udev_from_ifcfg() {
    local SCRIPTS_DIR=""

    # Detect the correct path: RHEL/CentOS vs SUSE
    if [ -d "$NETWORK_SCRIPTS_DIR" ]; then
        SCRIPTS_DIR="$NETWORK_SCRIPTS_DIR"
    elif [ -d "$NETWORK_SCRIPTS_DIR_SUSE" ]; then
        SCRIPTS_DIR="$NETWORK_SCRIPTS_DIR_SUSE"
    else
        log "Info: no ifcfg directory found (checked $NETWORK_SCRIPTS_DIR and $NETWORK_SCRIPTS_DIR_SUSE)."
        return 0
    fi

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read -r line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Find the matching network script file
        IFCFG=$(grep -l "IPADDR[0-9]*=.*$S_IP\b" "$SCRIPTS_DIR"/ifcfg-* 2>/dev/null)
        if [ -z "$IFCFG" ]; then
            log "Info: no ifcfg config file found for $S_IP in $SCRIPTS_DIR."
            continue
        fi

        # Extract device name from ifcfg file
        # RHEL/CentOS: typically has DEVICE= or HWADDR= inside the file
        # SUSE: device name is encoded in the filename itself (ifcfg-eth0 -> eth0)
        DEVICE=$(get_device_from_ifcfg "$IFCFG" "$S_HW")
        if [ -z "$DEVICE" ]; then
            # SUSE style: extract device name from filename (ifcfg-eth0 -> eth0)
            DEVICE=$(basename "$IFCFG" | sed 's/^ifcfg-//')
        fi

        if [ -z "$DEVICE" ] || [ "$DEVICE" = "lo" ]; then
            log "Info: no valid interface name found in $IFCFG."
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done
}

# Create udev rules based on the macToip mapping + network manager connections
udev_from_nm() {
    # Check if the network connections directory exists
    if [ ! -d "$NETWORK_CONNECTIONS_DIR" ]; then
        log "Warning: Directory $NETWORK_CONNECTIONS_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read -r line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Find the matching NetworkManager connection file
        NM_FILE=$(grep -El "address[0-9]*=.*$S_IP\b" "$NETWORK_CONNECTIONS_DIR"/*)
        if [ -z "$NM_FILE" ]; then
            log "Info: no nm config file name found for $S_IP."
            continue
        fi

        # Extract the DEVICE (interface name) from the matching file
        DEVICE=$(grep '^interface-name=' "$NM_FILE" | cut -d'=' -f2)
        if [ -z "$DEVICE" ]; then
            log "Info: no interface name found to $S_IP."
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done
}

# Attempt to parse the `timestamps` file and find a matching timestamp for the
# given UUID. The `timestamps` file is an 'ini'-like file with a format like:
#   [timestamps]
#   UUID1=TIMESTAMP1
#   UUID2=TIMESTAMP2
#   ...
#
get_timestamp_for_uuid() {
    TIMESTAMPS_FILE="$NM_LEASES_DIR/timestamps"

    if [ ! -f "$TIMESTAMPS_FILE" ]; then
        log "Warning: Timestamps file '$TIMESTAMPS_FILE' not found."
        echo "" # Return empty string
        return
    fi

    # Read the timestamps file line by line
    # Expected format: uuid=timestamp
    while IFS='=' read -r UUID TIMESTAMP; do
        # Skip header lines like "[timestamps]"
        [ "$UUID" = "[timestamps]" ] && continue
        # Skip empty lines or lines not in the expected key=value format
        [ -z "$UUID" ] || [ -z "$TIMESTAMP" ] && continue

        if [ "$UUID" = "$1" ]; then
            echo "$TIMESTAMP"
            break # UUID found, no need to read further
        fi
    done < "$TIMESTAMPS_FILE"
}

udev_from_nm_dhcp_lease() {
    if [ ! -d "$NM_LEASES_DIR" ]; then
        log "Warning: Directory $NM_LEASES_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    while read -r line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # find all lease files that mention the given address
        LEASE_FILES=$(grep -El "ADDRESS=$S_IP$" "$NM_LEASES_DIR"/*.lease)
        if [ -z "$LEASE_FILES" ]; then
            log "Warning: No lease files found containing address $S_IP"
            continue
        fi

        # parse the filenames of the matching lease files and grab the device name of
        # the most recent one
        DEVICE=$(for FILENAME in $LEASE_FILES;
        do
            log "Checking $FILENAME"
            # Filenames are of the form 'prefix-$(UUID)-$(INTERFACE_NAME).lease'
            FILENAME_PARTS=$(echo "$FILENAME" | sed -n 's|^.*-\([0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}\)-\(.*\)\.lease$|\1 \2|p')
            if [ -n "$FILENAME_PARTS" ]; then
                UUID=$(echo "$FILENAME_PARTS" | cut -d' ' -f1)
                INTERFACE=$(echo "$FILENAME_PARTS" | cut -d' ' -f2)
                TIMESTAMP=$(get_timestamp_for_uuid "$UUID")
                if [ -n "$TIMESTAMP" ]; then
                    echo "$TIMESTAMP $INTERFACE"
                else
                    log "Warning: No timestamp found for UUID '$UUID' from file '$FILENAME'"
                    echo "0 $INTERFACE"
                fi
            else
                log "Warning: Could not parse UUID/Interface from filename '$FILENAME'"
            fi
        done |sort -nr |head -1 |cut -d' ' -f2)

        if [ -z "$DEVICE" ]; then
            log "Warning: No device found for $S_IP"
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done < "$V2V_MAP_FILE"
}

udev_from_dhclient_lease() {
    local LEASE_DIRS=""
    [ -d "$DHCLIENT_LEASES_DIR" ] && LEASE_DIRS="$DHCLIENT_LEASES_DIR"
    [ -d "$NM_LEASES_DIR" ] && LEASE_DIRS="$LEASE_DIRS $NM_LEASES_DIR"

    if [ -z "$LEASE_DIRS" ]; then
        log "Warning: No dhclient lease directories found (checked $DHCLIENT_LEASES_DIR and $NM_LEASES_DIR)."
        return 0
    fi

    # Read the mapping file line by line
    while read -r line; do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        LATEST_EPOCH=0
        DEVICE=""

        # lease files in the dhclient are of the format:
        # lease {
        #   interface "eth0";
        #   fixed-address 192.168.122.82;
        #   ...
        #   expire <DAYOFWEEK> <DATE:Y/M/D> <TIME:H:M:S>;
        # }
        # Loop over each lease file and find the interface name associated with
        # S_IP that has the latest expiration date
        local CURRENT_INTERFACE=""
        local CURRENT_IP=""
        local CURRENT_EXPIRE=""
        local LATEST_EPOCH=0
        local DEVICE=""
        for DIR in $LEASE_DIRS; do
        for FILE in "$DIR"/dhclient-*; do
            [ -f "$FILE" ] || continue
            while IFS= read -r line || [ -n "$line" ]; do
                # Remove leading spaces
                line=$(echo "$line" | sed -e 's/^[[:space:]]*//' -e 's/;[[:space:]]*$//')
                # log "Processing line $line"
                case "$line" in
                    'interface'*)
                        # Extract interface name
                        CURRENT_INTERFACE=$(echo "$line" | sed -n 's/.*"\(.*\)".*/\1/p')
                        # log "Found device $CURRENT_DEVICE"
                        ;;
                    'expire'*)
                        # Extract and convert the date to epoch time
                        CURRENT_EXPIRE=$(echo "$line" | awk '{print $3, $4}')
                        # log "Found expire $EXPIRE"
                        ;;
                    'fixed-address'*)
                        # Extract and convert the date to epoch time
                        CURRENT_IP=$(echo "$line" | awk '{print $2}')
                        # log "Found expire $EXPIRE"
                        ;;
                    '}')
                        log "Processing block: $CURRENT_INTERFACE $CURRENT_EXPIRE"
                        if [ -n "$CURRENT_IP" ] && [ -n "$CURRENT_INTERFACE" ] && [ -n "$CURRENT_EXPIRE" ]; then
                            if [ "$S_IP" = "$CURRENT_IP" ]; then
                                epoch=$(date -d "$CURRENT_EXPIRE" +%s 2>/dev/null)
                                log "Found epoch $epoch"
                                if [ -n "$epoch" ] && [ "$epoch" -gt "$LATEST_EPOCH" ]; then
                                    log "$CURRENT_INTERFACE has the current latest epoch"
                                    LATEST_EPOCH=$epoch
                                    DEVICE=$CURRENT_INTERFACE
                                fi
                            else
                                log "Skipping block because $CURRENT_IP != $S_IP"
                            fi
                        fi
                        # reset for next block
                        CURRENT_IP=""
                        CURRENT_INTERFACE=""
                        CURRENT_EXPIRE=""
                        ;;
                esac
            done < "$FILE"
        done
        done

        if [ -z "$DEVICE" ]; then
            log "WARNING: No lease found for IP $S_IP"
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done < "$V2V_MAP_FILE"
}

# Create udev rules based on the macToIP mapping + output from parse_netplan_file
udev_from_netplan() {
    # Check if netplan command exist
    if ! ${IN_TESTING:-false} && ! command -v netplan >/dev/null 2>&1; then
        log "Warning: netplan is not installed."
        return 0
    fi

    # Function to check if netplan supports the 'get' subcommand
    netplan_supports_get() {
        if ${DISABLE_NETPLAN_GET:-false}; then
            return 1
        fi
        netplan get --root-dir "$NETPLAN_DIR" >&3
        return $?
    }

    # netplan with root dir
    netplan_get() {
        netplan get --root-dir "$NETPLAN_DIR" "$@" 2>&3
    }

    # Loop over all interface names and return the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"
        if netplan_supports_get; then
          # Loop through all interfaces and check for the given IP address
          netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read -r IFNAME; do
              if netplan_get ethernets."$IFNAME".addresses | grep -q "$target_ip\b"; then
                  echo "$IFNAME"
                  return
              fi
          done
        else
            if [ -z "$SYSTEMD_NETWORK_DIR" ]; then
                log "Info: no systemd network directory"
                return
            fi
            netplan generate --root-dir "$NETPLAN_DIR" 2>&3
            NM_FILE=$(grep -El "Address[0-9]*=.*$S_IP\b" "$SYSTEMD_NETWORK_DIR"/*)
            if [ -z "$NM_FILE" ]; then
                log "Info: no systemd nm config file name found for $S_IP."
                return
            fi
            # Extract the interface name from the matching file
            NAME=$(grep '^Name=' "$NM_FILE" | cut -d'=' -f2)
            if [ -z "$NAME" ]; then
                log "Info: no interface name found to $S_IP."
            fi
            echo "$NAME"
        fi
    }

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read -r line;
    do
        # Extract S_HW and S_IP from the current line in the mapping file
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Search the parsed netplan output for a matching IP address
        interface_name=$(find_interface_by_ip "$S_IP")

        # If no interface is found, skip this entry
        if [ -z "$interface_name" ]; then
            log "Info: no interface name found to $S_IP."
            continue
        fi

        # Create the udev rule based on the extracted MAC address and interface name
        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$interface_name")\""
    done
}

# Create udev rules based on the macToIP mapping + output from parse_ifquery_file
udev_from_ifquery() {
    # Check if ifquery command exist
    if ! ${IN_TESTING:-false} && ! command -v $IFQUERY_CMD>/dev/null 2>&1; then
        log "Warning: ifquery is not installed."
        return 0
    fi

    # ifquery with interface dir
    ifquery_get() {
        $IFQUERY_CMD -i "$NETWORK_INTERFACES_DIR" "$@" 2>&3
    }

    list_ifquery_interfaces() {
        # Interfaces may belong to more than one allow class, so keep the first
        # occurrence while preserving ifquery's order.
        {
            ifquery_get -l
            ifquery_get -l --allow=hotplug
        } | awk 'NF && !seen[$0]++'
        return $?
    }

    # Loop over all interface names and return the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"
        # Loop through all interfaces and check for the given IP address
        list_ifquery_interfaces | while read -r IFNAME; do
            if ifquery_get "$IFNAME" | grep -q "$target_ip\b"; then
                echo "$IFNAME"
                return
            fi
        done
    }

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read -r line;
    do
        # Extract S_HW and S_IP from the current line in the mapping file
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Search the parsed ifquery output for a matching IP address
        interface_name=$(find_interface_by_ip "$S_IP")

        # If no interface is found, skip this entry
        if [ -z "$interface_name" ]; then
            log "Info: no interface name found to $S_IP."
            continue
        fi

        # Create the udev rule based on the extracted MAC address and interface name
        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$interface_name")\""
    done
}

# Generate systemd .link files to ensure interface renaming works on all distros.
# On RHEL 9/10+ predictable naming overrides raw udev NAME= rules, but a .link
# file with a lower priority number takes precedence over the default policy.
# On older distros the .link file is redundant with the udev rule but harmless.
#
# .link files are only created when a NetworkManager connection profile exists
# for the IP but does not already contain the correct mac-address= binding.
generate_link_files() {
    # Only run on NM-based distros,skip Ubuntu/Debian/SUSE where this dir is absent.
    if [ ! -d "$NETWORK_CONNECTIONS_DIR" ]; then
        log "Info: $NETWORK_CONNECTIONS_DIR not found, skipping .link file creation."
        return 0
    fi

    local CREATED=false

    cat "$V2V_MAP_FILE" | while read -r line; do
        extract_mac_ip "$line"

        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            continue
        fi

        # Find the NM connection profile that owns this IP.
        local NM_FILE
        NM_FILE=$(grep -El "address[0-9]*=.*$S_IP\b" "$NETWORK_CONNECTIONS_DIR"/* 2>/dev/null | head -1)
        if [ -z "$NM_FILE" ]; then
            continue
        fi

        # Read the interface name the connection is bound to.
        local DEVICE
        DEVICE=$(grep '^interface-name=' "$NM_FILE" | cut -d'=' -f2)
        if [ -z "$DEVICE" ]; then
            continue
        fi

        local MAC_LOWER
        MAC_LOWER=$(echo "$S_HW" | tr 'A-F' 'a-f')

        # If the profile already carries the correct MAC, a .link file is redundant.
        local EXISTING_MAC
        EXISTING_MAC=$(grep '^mac-address=' "$NM_FILE" | cut -d'=' -f2 | tr 'A-F' 'a-f')
        if [ "$EXISTING_MAC" = "$MAC_LOWER" ]; then
            log "Info: $NM_FILE already contains mac-address=$MAC_LOWER, skipping .link for $DEVICE."
            continue
        fi

        # Create the target directory only when the first .link file is needed.
        if [ "$CREATED" = false ]; then
            mkdir -p "$SYSTEMD_LINK_DIR"
            CREATED=true
        fi

        # Write the .link file: match by MAC, rename to the original interface name.
        local LINK_FILE="$SYSTEMD_LINK_DIR/10-v2v-${DEVICE}.link"
        printf '[Match]\nMACAddress=%s\n\n[Link]\nName=%s\n' "$MAC_LOWER" "$DEVICE" > "$LINK_FILE"
        log "Created $LINK_FILE: $MAC_LOWER -> $DEVICE"
    done
}

# Write to udev config
# ----------------------------------------

# Dedup identical rules (multi-IP NICs) and reject same-MAC-different-NAME conflicts.
check_dupe_hws() {
    input=$(cat)

    deduped=$(echo "$input" | awk '!seen[$0]++')

    # If the same MAC still appears in more than one rule, the NAMEs differ — conflict.
    conflicts=$(echo "$deduped" | grep -ioE "[0-9A-F:]{17}" | tr 'a-f' 'A-F' | sort | uniq -d)

    if [ -n "$conflicts" ]; then
        log "Warning: Conflicting rules for same MAC with different names: $conflicts"
        return 0
    fi

    echo "$deduped"
}

# Create udev rules check for duplicates and write them to udev file
main() {
    {
        udev_from_ifcfg
        udev_from_nm
        udev_from_nm_dhcp_lease
        udev_from_dhclient_lease
        udev_from_netplan
        udev_from_ifquery
        udev_from_wicked
    } | check_dupe_hws > "$UDEV_RULES_FILE" 2>/dev/null
    echo "New udev rule:"
    cat $UDEV_RULES_FILE

    generate_link_files
}

main
fix_pod_network_ipv6
