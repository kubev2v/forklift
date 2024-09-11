#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"
NETPLAN_DIR="${NETPLAN_DIR:-/}"

# Dump debug strings into a new file descriptor and redirect it to stdout.
exec 3>&1
log() {
    echo $@ >&3
}

# Sanity checks
# -------------

# Check if mapping file does not exist
if [ ! -f "$V2V_MAP_FILE" ]; then
    log "File $V2V_MAP_FILE does not exist. Exiting."
    exit 0
fi

# Check if udev rules file exists
if [ -f "$UDEV_RULES_FILE" ]; then
    log "File $UDEV_RULES_FILE already exists. Exiting."
    exit 0
fi

# Helper functions
# ----------------

# Clean strigs in case they have quates
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

# Create udev rules based on the macToip mapping + ifcfg network scripts
udev_from_ifcfg() {
    # Check if the network scripts directory exists
    if [ ! -d "$NETWORK_SCRIPTS_DIR" ]; then
        log "Warning: Directory $NETWORK_SCRIPTS_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            continue
        fi

        # Find the matching network script file
        IFCFG=$(grep -l "IPADDR=$S_IP" "$NETWORK_SCRIPTS_DIR"/*)
        if [ -z "$IFCFG" ]; then
            continue
        fi

        # Source the matching file, if found
        DEVICE=$(grep '^DEVICE=' "$IFCFG" | cut -d'=' -f2)
        if [ -z "$DEVICE" ]; then
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
    cat "$V2V_MAP_FILE" | while read line;
    do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            continue
        fi

        # Find the matching NetworkManager connection file
        NM_FILE=$(grep -El "address[0-9]*=$S_IP" "$NETWORK_CONNECTIONS_DIR"/*)
        if [ -z "$NM_FILE" ]; then
            continue
        fi

        # Extract the DEVICE (interface name) from the matching file
        DEVICE=$(grep '^interface-name=' "$NM_FILE" | cut -d'=' -f2)
        if [ -z "$DEVICE" ]; then
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$DEVICE")\""
    done
}

# Create udev rules based on the macToIP mapping + output from parse_netplan_file
udev_from_netplan() {
    # Check if netplan command exist
    if ! command -v netplan >/dev/null 2>&1; then
        log "Warning: netplan is not installed."
        return 0
    fi

    # netplan with root dir
    netplan_get() {
        netplan get --root-dir "$NETPLAN_DIR" "$@" 2>/dev/null
    }

    # Loop over all interface names and treturn the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"

        # Loop through all interfaces and check for the given IP address
        netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read IFNAME; do
            if netplan_get ethernets."$IFNAME".addresses | grep -q "$target_ip"; then
                echo "$IFNAME"
            fi
        done
    }

    # Read the mapping file line by line
    cat "$V2V_MAP_FILE" | while read line;
    do
        # Extract S_HW and S_IP from the current line in the mapping file
        extract_mac_ip "$line"

        # If S_HW and S_IP were not extracted, skip the line
        if [ -z "$S_HW" ] || [ -z "$S_IP" ]; then
            continue
        fi

        # Search the parsed netplan output for a matching IP address
        interface_name=$(find_interface_by_ip "$S_IP")

        # If no interface is found, skip this entry
        if [ -z "$interface_name" ]; then
            continue
        fi

        # Create the udev rule based on the extracted MAC address and interface name
        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(remove_quotes "$S_HW")\",NAME=\"$(remove_quotes "$interface_name")\""
    done
}

# Write to udev config
# ----------------------------------------

# Checks for duplicate hardware addresses 
check_dupe_hws() {
    input=$(cat)

    # Extract MAC addresses, convert to uppercase, sort them, and find duplicates
    dupes=$(echo "$input" | grep -ioE "[0-9A-F:]{17}" | tr 'a-f' 'A-F' | sort | uniq -d)

    # If duplicates are found, print an error and exit
    if [ -n "$dupes" ]; then
        log "Warning: Duplicate hw: $dupes"
        return 0
    fi

    echo "$input"
}

# Create udev rules check for duplicates and write them to udev file
main() {
    {
        udev_from_ifcfg
        udev_from_nm
        udev_from_netplan
    } | check_dupe_hws > "$UDEV_RULES_FILE" 2>/dev/null
}

main
