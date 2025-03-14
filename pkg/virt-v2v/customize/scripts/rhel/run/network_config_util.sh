#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
NETWORK_INTERFACES_DIR="${NETWORK_INTERFACES_DIR:-/etc/network/interfaces}"
IFQUERY_CMD="${IFQUERY_CMD:-ifquery}"
SYSTEMD_NETWORK_DIR="${SYSTEMD_NETWORK_DIR:-/run/systemd/network}"
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
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Find the matching network script file
        IFCFG=$(grep -l "IPADDR=.*$S_IP.*$" "$NETWORK_SCRIPTS_DIR"/*)
        if [ -z "$IFCFG" ]; then
            log "Info: no ifcg config file name found for $S_IP."
            continue
        fi

        # Source the matching file, if found
        DEVICE=$(get_device_from_ifcfg "$IFCFG" "$S_HW")
        if [ -z "$DEVICE" ]; then
            log "Info: no interface name found to $S_IP in $IFCFG."
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
            log "Warning: invalid mac to ip line: $line."
            continue
        fi

        # Find the matching NetworkManager connection file
        NM_FILE=$(grep -El "address[0-9]*=.*$S_IP.*$" "$NETWORK_CONNECTIONS_DIR"/*)
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
        netplan get >&3
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
          netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read IFNAME; do
              if netplan_get ethernets."$IFNAME".addresses | grep -q "$target_ip"; then
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
            NM_FILE=$(grep -El "Address[0-9]*=.*$S_IP.*$" "$SYSTEMD_NETWORK_DIR"/*)
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
    cat "$V2V_MAP_FILE" | while read line;
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

    # Loop over all interface names and return the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"
        # Loop through all interfaces and check for the given IP address
        ifquery_get -l | while read IFNAME; do
            if ifquery_get $IFNAME | grep -q "$target_ip"; then
                echo "$IFNAME"
                return
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
        udev_from_ifquery
    } | check_dupe_hws > "$UDEV_RULES_FILE" 2>/dev/null
    echo "New udev rule:"
    cat $UDEV_RULES_FILE
}

main
