#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"

# Check if udev rules file exists
if [ -f "$UDEV_RULES_FILE" ]; then
    echo "File $UDEV_RULES_FILE already exists. Exiting."
    exit 0
fi

# Add log file descriptot and dump it to stdout
exec 3>&1
log() {
    echo $@ >&3
}

# Validate MAC address and IPv4 address and extract them
extract_mac_ip() {
    S_HW=""
    S_IP=""
    if [[ "$1" =~ ^([0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}):ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}) ]]; then
        S_HW="${BASH_REMATCH[1]}"
        S_IP="${BASH_REMATCH[2]}"
    fi
}

# Create udev rules based on the macToip mapping + ifcfg network scripts
udev_from_ifcfg() {
    # Read the mapping file line by line
    while IFS= read -r line; do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"
        
        # If S_HW and S_IP were extracted, proceed
        [[ -z  "$S_HW" || -z "$S_IP" ]] && continue

        # Source the matching file, if found
        IFCFG=$(grep -l "IPADDR=$S_IP" "$NETWORK_SCRIPTS_DIR"/*)
        [[ -z "$IFCFG" ]] && continue
        source "$IFCFG"

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$S_HW\",NAME=\"$DEVICE\""

    done < "$V2V_MAP_FILE"
}

# Create udev rules based on the macToip mapping + network manager connections
udev_from_nm() {
    # Read the mapping file line by line
    while IFS= read -r line; do
        # Extract S_HW and S_IP
        extract_mac_ip "$line"

        # If S_HW and S_IP were extracted, proceed
        [[ -z  "$S_HW" || -z "$S_IP" ]] && continue

        # Find the matching NetworkManager connection file
        NM_FILE=$(grep -El "address[0-9]*=$S_IP" "$NETWORK_CONNECTIONS_DIR"/*)
        [[ -z "$NM_FILE" ]] && continue

        # Extract the DEVICE (interface name) from the matching file
        DEVICE=$(grep -oP '^interface-name=\K.*' "$NM_FILE")
        [[ -z "$DEVICE" ]] && continue

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$S_HW\",NAME=\"$DEVICE\""
    done < "$V2V_MAP_FILE"
}

# Checks for duplicate hardware addresses 
check_dupe_hws() {
    input=$(cat)

    # Extract MAC addresses, convert to uppercase, sort them, and find duplicates
    dupes=$(grep -io -E "[0-9A-F:]{17}" <<< "$input" | tr 'a-f' 'A-F' | sort | uniq -d)

    # If duplicates are found, print an error and exit
    if [ -n "$dupes" ]; then
        echo "ERR: Duplicate hw: $dupes"
        exit 2
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
