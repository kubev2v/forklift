#!/bin/bash

# Global variables with default values
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_SCRIPTS_DIR_SUSE="${NETWORK_SCRIPTS_DIR_SUSE:-/etc/sysconfig/network}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
NM_LEASES_DIR="${NM_LEASES_DIR:-/var/lib/NetworkManager}"
DHCLIENT_LEASES_DIR="${DHCLIENT_LEASES_DIR:-/var/lib/dhclient}"
NETWORK_INTERFACES_DIR="${NETWORK_INTERFACES_DIR:-/etc/network/interfaces}"
IFQUERY_CMD="${IFQUERY_CMD:-ifquery}"
SYSTEMD_NETWORK_DIR="${SYSTEMD_NETWORK_DIR:-/run/systemd/network}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"
NETPLAN_DIR="${NETPLAN_DIR:-/}"

# Dump debug strings into a new file descriptor and redirect it to stdout.
exec 3>&1
log() {
    echo "$@" >&3
}

# Sanity checks
# -------------

# Check if mapping file does not exist
if [ ! -f "$V2V_MAP_FILE" ]; then
    log "File $V2V_MAP_FILE does not exist. Exiting."
    exit 0
fi

# Check if udev rules file exists and is not empty
if [ -f "$UDEV_RULES_FILE" ] && [ -s "$UDEV_RULES_FILE" ]; then
    log "File $UDEV_RULES_FILE already exists and is not empty. Exiting."
    exit 0
fi

# Helper functions
# ----------------

# Clean strings in case they have quotes
remove_quotes() {
    echo "$1" | tr -d '"' | tr -d "'" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

# Extract MAC and IP (IPv4/IPv6) from a line in the mapping file
extract_mac_ip() {
    S_HW=""
    S_IP=""
    line="$1"
    S_HW=$(echo "$line" | sed -n 's/^\([0-9a-fA-F:]\{17\}\):ip:.*/\1/p')
    S_IP=$(echo "$line" | sed -n 's/^.*:ip:\([^,]*\).*/\1/p')
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

        # Find the matching network script file (check both IPv4 IPADDR= and IPv6 IPV6ADDR=)
        IFCFG=$(grep -l -E "(IPADDR|IPV6ADDR)=.*$S_IP" "$SCRIPTS_DIR"/ifcfg-* 2>/dev/null)
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

        # find all lease files that mention the given address (IPv4 or IPv6)
        # Use -w for word boundary matching to avoid partial IP matches (e.g., 192.168.1.1 vs 192.168.1.10)
        LEASE_FILES=$(grep -l -w -F "$S_IP" "$NM_LEASES_DIR"/*.lease 2>/dev/null)
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
    if [ ! -d "$DHCLIENT_LEASES_DIR" ]; then
        log "Warning: Directory $DHCLIENT_LEASES_DIR does not exist."
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
        for FILE in "$DHCLIENT_LEASES_DIR"/*; do
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
          netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read -r IFNAME; do
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
            # Check for both IPv4 and IPv6 address patterns in systemd-networkd config
            NM_FILE=$(grep -l -w -F "$S_IP" "$SYSTEMD_NETWORK_DIR"/* 2>/dev/null)
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

    # Loop over all interface names and return the one with target_ip, or null
    find_interface_by_ip() {
        target_ip="$1"
        # Loop through all interfaces and check for the given IP address
        ifquery_get -l | while read -r IFNAME; do
            if ifquery_get $IFNAME | grep -q "$target_ip"; then
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

# Write to udev config
# ----------------------------------------

# Checks for duplicate hardware addresses 
check_dupe_hws() {
    awk '
    {
        print
        mac = ""
        name = ""
        # Extract MAC from ATTR{address}=="..." using portable split/substr
        if (index($0, "ATTR{address}==\"") > 0) {
            split($0, _a, "ATTR{address}==\"")
            split(_a[2], _b, "\"")
            mac = tolower(_b[1])
        }
        # Extract name from NAME=="..."
        if (index($0, "NAME==\"") > 0) {
            split($0, _c, "NAME==\"")
            split(_c[2], _d, "\"")
            name = _d[1]
        }
        if (mac && name) {
            if ((mac in seen) && seen[mac] != name) {
                printf "Error: MAC %s assigned to multiple interface names: %s and %s\n", mac, seen[mac], name > "/dev/stderr"
                exit_code = 1
            } else {
                seen[mac] = name
            }
        }
    }
    END { exit(exit_code) }
    '
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
    } | awk '!seen[$0]++' | check_dupe_hws > "$UDEV_RULES_FILE"

    echo "New udev rule:"
    cat $UDEV_RULES_FILE
}

main
