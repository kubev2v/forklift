#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"

# Check if mapping file does not exist
if [ ! -f "$V2V_MAP_FILE" ]; then
    echo "File $V2V_MAP_FILE does not exist. Exiting."
    exit 0
fi

# Check if udev rules file exists
if [ -f "$UDEV_RULES_FILE" ]; then
    echo "File $UDEV_RULES_FILE already exists. Exiting."
    exit 0
fi

# Validate MAC address and IPv4 address and extract them
extract_mac_ip() {
    S_HW=""
    S_IP=""
    if echo "$1" | grep -qE '^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$'; then
        S_HW=$(echo "$1" | sed -nE 's/^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:.*$/\1/p')
        S_IP=$(echo "$1" | sed -nE 's/^.*:ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$/\1/p')
    fi
}

# Create udev rules based on the macToip mapping + ifcfg network scripts
udev_from_ifcfg() {
    # Check if the network scripts directory exists
    if [ ! -d "$NETWORK_SCRIPTS_DIR" ]; then
        echo "Warning: Directory $NETWORK_SCRIPTS_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    while IFS= read -r line; do
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

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$S_HW\",NAME=\"$DEVICE\""
    done < "$V2V_MAP_FILE"
}

# Create udev rules based on the macToip mapping + network manager connections
udev_from_nm() {
    # Check if the network connections directory exists
    if [ ! -d "$NETWORK_CONNECTIONS_DIR" ]; then
        echo "Warning: Directory $NETWORK_CONNECTIONS_DIR does not exist."
        return 0
    fi

    # Read the mapping file line by line
    while IFS= read -r line; do
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

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$S_HW\",NAME=\"$DEVICE\""
    done < "$V2V_MAP_FILE"
}

# Checks for duplicate hardware addresses 
check_dupe_hws() {
    input=$(cat)

    # Extract MAC addresses, convert to uppercase, sort them, and find duplicates
    dupes=$(echo "$input" | grep -ioE "[0-9A-F:]{17}" | tr 'a-f' 'A-F' | sort | uniq -d)

    # If duplicates are found, print an error and exit
    if [ -n "$dupes" ]; then
        echo "Warning: Duplicate hw: $dupes"
        return 0
    fi

    echo "$input"
}

# Create udev rules check for duplicates and write them to udev file
main() {
    {
        udev_from_ifcfg
        udev_from_nm
    } | check_dupe_hws > "$UDEV_RULES_FILE"
}

main
