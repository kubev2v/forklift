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

# Get a netplan setting by specifying a nested key like "ethernets.eth0.addresses"
# For example:
#    netplan_get_py ethernets
# Will return the yaml struct of all the thernet interfaces.
netplan_get_py() {
    python -c "
import os
import yaml
import sys

netplan_dir = os.getenv('NETPLAN_DIR', '') + '/etc/netplan'
args = sys.argv[1].split('.')

def find_yaml_files(directory):
    yaml_files = []
    for root, dirs, files in os.walk(directory):
        for file in files:
            if file.endswith(('.yaml', '.yml')):
                yaml_files.append(os.path.join(root, file))

    return yaml_files

def get_yaml_path(yaml_data, keys):
    for key in keys:
        if yaml_data is None:
            return None
        yaml_data = yaml_data.get(key)
    return yaml_data

yaml_files = find_yaml_files(netplan_dir)

for yaml_file in yaml_files:
    with open(yaml_file, 'r') as file:
        yaml_data = yaml.safe_load(file)        
        result = get_yaml_path(yaml_data, ['network'] + args)
        if result is not None:
           print(yaml.dump(result, default_flow_style=False))
" "$@"
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
            log "Warning: invalide mac to ip line: $line."
            continue
        fi

        # Find the matching network script file
        IFCFG=$(grep -l "IPADDR=$S_IP" "$NETWORK_SCRIPTS_DIR"/*)
        if [ -z "$IFCFG" ]; then
            log "Info: no ifcg config file name foud for $S_IP."
            continue
        fi

        # Source the matching file, if found
        DEVICE=$(grep '^DEVICE=' "$IFCFG" | cut -d'=' -f2)
        if [ -z "$DEVICE" ]; then
            log "Info: no interface name found to $S_IP."
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
            log "Warning: invalide mac to ip line: $line."
            continue
        fi

        # Find the matching NetworkManager connection file
        NM_FILE=$(grep -El "address[0-9]*=$S_IP" "$NETWORK_CONNECTIONS_DIR"/*)
        if [ -z "$NM_FILE" ]; then
            log "Info: no nm config file name foud for $S_IP."
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
    if ! command -v netplan >/dev/null 2>&1; then
        log "Warning: netplan is not installed."
        return 0
    fi

    # Function to check if netplan supports the 'get' subcommand
    netplan_supports_get() {
        netplan get 2>/dev/null
        return $?
    }

    # netplan with root dir
    netplan_get() {
        if netplan_supports_get; then
            netplan get --root-dir "$NETPLAN_DIR" "$@" 2>&3
        else
            log 'Info: netplan not supporting get subcomment, using python'
            netplan_get_py "$@" 2>&3
        fi
    }

    # Loop over all interface names and treturn the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"

        # Loop through all interfaces and check for the given IP address
        netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read IFNAME; do
            if netplan_get ethernets."$IFNAME".addresses | grep -q "$target_ip"; then
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
            log "Warning: invalide mac to ip line: $line."
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
