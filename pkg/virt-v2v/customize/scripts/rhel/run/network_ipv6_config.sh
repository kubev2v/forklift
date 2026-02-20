#!/bin/bash

# network_ipv6_config.sh
#
# PURPOSE:
# --------
# This script configures IPv6 static addresses in network configuration files.
# It reads IPv6 addresses from /tmp/macToIP and adds them to the corresponding ifcfg files.
#
# BACKGROUND:
# -----------
# virt-v2v only configures IPv4. By setting IPV6_AUTOCONF=yes, it enables SLAAC which
# can override static IPv6 addresses we want to preserve from the source VM.
#
# This script complements virt-v2v by:
# 1. Reading IPv6 addresses from /tmp/macToIP (created by forklift controller)
# 2. Finding the corresponding ifcfg file using udev rules (MAC → device name mapping)
# 3. Adding IPv6ADDR, gateway, and DNS configuration
# 4. Setting IPV6_AUTOCONF=no to disable SLAAC for static IPs
#
# EXECUTION CONTEXT:
# ------------------
# This script runs via virt-customize after virt-v2v has completed, so ifcfg files and
# udev rules already exist. It modifies the ifcfg files in place before the VM first boots.
#
# INPUT FORMAT (/tmp/macToIP):
# ----------------------------
# Each line: MAC:ip:IP,GATEWAY,PREFIX,DNS1,DNS2,...
# Example: 00:50:56:a0:e3:ff:ip:2620:52:9:162e::1234,fe80::1,64,2620:52:9:162e::53

# =============================================================================
# Configuration and Setup
# =============================================================================

# Global variables with default values (can be overridden for testing)
V2V_MAP_FILE="${V2V_MAP_FILE:-/tmp/macToIP}"
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
UDEV_RULES_FILE="/etc/udev/rules.d/70-persistent-net.rules"

# Logging setup
# -------------
# Create a file descriptor (FD 3) for logging output
# This allows log messages to go to stdout while other operations stay quiet
exec 3>&1
log() {
    echo "$@" >&3
}

# =============================================================================
# Sanity Checks
# =============================================================================

# Check if mapping file exists
# Exit gracefully if not found (nothing to configure)
if [ ! -f "$V2V_MAP_FILE" ]; then
    log "File $V2V_MAP_FILE does not exist. Exiting."
    exit 0
fi

# Check if network scripts directory exists
# Exit gracefully if not found (unexpected but not an error)
if [ ! -d "$NETWORK_SCRIPTS_DIR" ]; then
    log "Directory $NETWORK_SCRIPTS_DIR does not exist. Exiting."
    exit 0
fi

# =============================================================================
# Helper Functions
# =============================================================================

# is_ipv6()
# ---------
# Check if an IP address is IPv6 by looking for colons
# IPv4 addresses use dots (e.g., 192.168.1.1)
# IPv6 addresses use colons (e.g., 2620:52:9:162e::1234)
#
# Args:
#   $1 - IP address string
# Returns:
#   0 (true) if IPv6, 1 (false) if not
is_ipv6() {
    local ip="$1"
    [[ "$ip" == *:* ]]
}

# =============================================================================
# Main Configuration Function
# =============================================================================

# configure_ipv6_in_ifcfg()
# -------------------------
# Main function that processes /tmp/macToIP and adds IPv6 configuration to ifcfg files
#
# Process:
#   For each IPv6 entry in /tmp/macToIP:
#     1. Extract MAC address and IPv6 configuration (IP, gateway, prefix, DNS)
#     2. Find device name from udev rules using MAC address
#     3. Locate the ifcfg file for that device
#     4. Set IPV6_AUTOCONF=no to disable SLAAC
#     5. Add IPv6 address, gateway, and DNS entries to the ifcfg file
#
# The function is idempotent - safe to run multiple times without duplicating configuration
configure_ipv6_in_ifcfg() {
    log "Adding IPv6 configuration from $V2V_MAP_FILE to ifcfg files..."
    
    # Read each line from the macToIP file
    # Each line format: MAC:ip:IP,GATEWAY,PREFIX,DNS1,DNS2,...
    while IFS= read -r line; do
        # Parse the line format: MAC:ip:IP,GATEWAY,PREFIX,DNS1,DNS2
        # Example: 00:50:56:a0:e3:ff:ip:2620:52:9:162e::1234,2620:52:9:162e::1,64,2620:52:9:162e::53
        
        # Extract MAC address (first 17 chars) and rest of the line using regex
        # Regex: ^([0-9a-fA-F:]{17}):ip:(.+)$
        #   - ^([0-9a-fA-F:]{17}) = MAC address at start (e.g., 00:50:56:a0:e3:ff)
        #   - :ip: = literal separator
        #   - (.+)$ = rest of line (IP configuration)
        if [[ ! "$line" =~ ^([0-9a-fA-F:]{17}):ip:(.+)$ ]]; then
            continue  # Skip malformed lines
        fi
        
        S_MAC="${BASH_REMATCH[1]}"    # Extracted MAC address
        S_REST="${BASH_REMATCH[2]}"   # IP configuration string
        
        # Split IP configuration by comma into array
        # Example: "2620:52:9:162e::1234,fe80::1,64," -> ["2620:52:9:162e::1234", "fe80::1", "64", ""]
        IFS=',' read -ra IP_PARTS <<< "$S_REST"
        
        # Validate we have at least IP, GATEWAY, and PREFIX (minimum 3 fields)
        if [ "${#IP_PARTS[@]}" -lt 3 ]; then
            continue  # Skip incomplete entries
        fi
        
        # Extract the configuration fields
        S_IP="${IP_PARTS[0]}"         # IP address
        S_GATEWAY="${IP_PARTS[1]}"    # Gateway address
        S_PREFIX="${IP_PARTS[2]}"     # Prefix length (e.g., 64)
        # DNS servers are in IP_PARTS[3], IP_PARTS[4], etc. (handled later)
        
        # Only process IPv6 addresses (skip IPv4 entries)
        # This script is specifically for adding IPv6 configuration
        if ! is_ipv6 "$S_IP"; then
            continue
        fi
        
        # Step 1: Find the network interface device name from MAC address
        # -------------------------------------------------------------
        # Problem: virt-v2v doesn't write HWADDR to ifcfg files, so we can't match directly
        # Solution: Use udev rules created by network_config_util.sh that map MAC → device name
        
        # Extract device name from udev rules for this MAC address
        DEVICE_NAME=""
        if [ -f "$UDEV_RULES_FILE" ]; then
            # Search for the udev rule containing this MAC address
            # Example udev rule line:
            # SUBSYSTEM=="net", ACTION=="add", DRIVERS=="?*", ATTR{address}=="00:50:56:b4:07:84", NAME="eth0"


            # - Case-insensitive match on ATTR{address}=="MAC"
            # - Extract NAME="device" field
            # - Exit after first match for efficiency
            DEVICE_NAME=$(awk -v mac="$S_MAC" '
                tolower($0) ~ "attr\\{address\\}==\""tolower(mac)"\"" {
                    if (match($0, /NAME="([^"]+)"/, a)) {
                        print a[1]; exit
                    }
                }' "$UDEV_RULES_FILE")
        fi
        
        # Validate that we found a device name
        if [ -z "$DEVICE_NAME" ]; then
            log "Warning: No device name found in udev rules for MAC $S_MAC (IPv6: $S_IP)"
            continue  # Skip this entry if we can't find the device
        fi
        
        # Step 2: Locate the ifcfg file for this device
        # ----------------------------------------------
        # Build the path: /etc/sysconfig/network-scripts/ifcfg-eth0
        IFCFG="$NETWORK_SCRIPTS_DIR/ifcfg-${DEVICE_NAME}"
        
        # Verify the ifcfg file exists
        if [ ! -f "$IFCFG" ]; then
            log "Warning: ifcfg file not found: $IFCFG for MAC $S_MAC (IPv6: $S_IP)"
            continue  # Skip if ifcfg file doesn't exist
        fi
        
        # Step 3: Check if IPv6 is already configured (idempotency)
        # ----------------------------------------------------------
        # If IPV6ADDR already exists, skip to avoid duplicate configuration
        if grep -q "^IPV6ADDR=" "$IFCFG" 2>/dev/null; then
            log "Info: IPv6 already configured in $IFCFG, skipping"
            continue  # Script is idempotent - safe to run multiple times
        fi
        
        log "Info: Adding IPv6 configuration to $IFCFG for $S_IP"
        
        # Step 4: Disable SLAAC (Stateless Address Autoconfiguration)
        # ------------------------------------------------------------
        # For static IPv6 addresses, we need to disable SLAAC by setting IPV6_AUTOCONF=no
        # Otherwise, the system might get dynamic IPv6 addresses from Router Advertisements
        # which could override or conflict with our static configuration
        if grep -q "^IPV6_AUTOCONF=" "$IFCFG" 2>/dev/null; then
            # IPV6_AUTOCONF exists (likely set to "yes" by virt-v2v), change it to "no"
            sed -i 's/^IPV6_AUTOCONF=.*/IPV6_AUTOCONF=no/' "$IFCFG"
        else
            # IPV6_AUTOCONF doesn't exist, add it
            echo 'IPV6_AUTOCONF=no' >> "$IFCFG"
        fi
        
        # Step 5: Add IPv6 address, gateway, and DNS configuration
        # ---------------------------------------------------------
        # Write all IPv6 configuration lines to the ifcfg file
        {
            # Add the IPv6 address with prefix length
            # Example: IPV6ADDR=2620:52:9:162e:647e:273d:cb4a:3e4/64
            echo "IPV6ADDR=${S_IP}/${S_PREFIX}"
            
            # Add IPv6 default gateway if provided
            # Gateway can be empty for some configurations (e.g., no default route)
            if [ -n "$S_GATEWAY" ]; then
                # Example: IPV6_DEFAULTGW=fe80::4a5a:d01:f431:3320
                echo "IPV6_DEFAULTGW=${S_GATEWAY}"
            fi
            
            # Add DNS servers if provided (positions 3+ in the array)
            # DNS entries are appended to existing DNS configuration
            if [ "${#IP_PARTS[@]}" -gt 3 ]; then
                # Count existing DNS entries in the file to avoid duplicates
                # DNS entries are numbered: DNS1=, DNS2=, DNS3=, etc.
                DNS_COUNT=$(grep -Ec "^DNS[0-9]+=" "$IFCFG" 2>/dev/null || echo 0)
                
                # Iterate through DNS servers (starting from index 3)
                for i in "${!IP_PARTS[@]}"; do
                    if [ "$i" -gt 2 ]; then
                        DNS="${IP_PARTS[$i]}"
                        # Only add non-empty DNS entries
                        if [ -n "$DNS" ]; then
                            DNS_COUNT=$(( DNS_COUNT + 1 ))
                            # Example: DNS3=2620:52:9:162e::53
                            echo "DNS${DNS_COUNT}=${DNS}"
                        fi
                    fi
                done
            fi
        } >> "$IFCFG"  # Append all lines to the ifcfg file
        
        log "Info: IPv6 configuration added to $IFCFG"
    done < "$V2V_MAP_FILE"
}

# Run the main configuration function
configure_ipv6_in_ifcfg

log "IPv6 configuration completed."



