package main

const CheckConnectivityBash = `#!/bin/bash

# Function to check network connectivity
check_connectivity() {
    local timeout=$1
    local tries=0

    # Check network connectivity using nmcli
    if conn=$(nmcli networking connectivity); then
        # Loop until connectivity is full or the maximum number of tries is reached
        while [ $tries -lt "$timeout" ] && [ "$conn" != "full" ]; do
            sleep 1
            tries=$((tries + 1))
            conn=$(nmcli networking connectivity)
        done
    # Fallback to systemd-networkd if nmcli check fails
    elif systemctl -q is-active systemd-networkd; then
        /usr/lib/systemd/systemd-networkd-wait-online -q --timeout="$timeout"
    else
        echo "Neither nmcli nor systemd-networkd are available to check connectivity."
        return 1
    fi

    # Final connectivity status
    if [ "$conn" = "full" ]; then
        echo "Network connectivity is full."
        return 0
    else
        echo "Network connectivity is not full."
        return 1
    fi
}

# Set default timeout to 60 if not provided
timeout=${1:-60}

# Call the function with the provided or default timeout
check_connectivity "$timeout"
`

const CopyConnectionsBash = `#!/bin/bash

# Function to check if required tools exist
check_tools() {
    for tool in ip nmcli; do
        if ! command -v $tool &> /dev/null; then
            echo "Error: $tool is not installed. Please install it and try again."
            exit 1
        fi
    done
}

# Function to identify the network device using "ip"
identify_device() {
    device=$(ip link show | awk -F: '/state UP/ {print $2}' | tr -d ' ')
    if [ -z "$device" ]; then
        echo "Error: No active network device found."
        exit 1
    fi
    echo "Active network device: $device"
}

# Function to identify the network connection connected to the device
identify_connection() {
    connection=$(nmcli -t -f NAME,DEVICE connection show --active | grep ":$device$" | cut -d: -f1)
    if [ -z "$connection" ]; then
        echo "Error: No active connection found for device $device."
        exit 1
    fi
    echo "Active network connection: $connection"
}

# Function to find an Ethernet connection that does not have a device connected
find_unconnected_connection() {
    unconnected_connection=$(nmcli -t -f NAME,TYPE,DEVICE connection show | grep ":$" | grep "ethernet" | cut -d: -f1)
    if [ -z "$unconnected_connection" ]; then
        echo "Error: No Ethernet connection without a connected device found."
        exit 1
    elif [ $(echo "$unconnected_connection" | wc -l) -ne 1 ]; then
        echo "Error: More than one Ethernet connection without a connected device found."
        exit 1
    fi
    echo "Unconnected Ethernet connection: $unconnected_connection"
}

# Function to copy IPv4 fields
copy_ipv4_fields() {
    echo "Copying IPv4 settings from $unconnected_connection to $connection..."

    # Initialize the nmcli command with the connection name
    nmcli_cmd="nmcli connection modify \"$connection\""

    # Get the ipv4.method
    ipv4_method=$(nmcli -g ipv4.method connection show "$unconnected_connection")
    if [ "$ipv4_method" != "--" ]; then
        nmcli_cmd+=" ipv4.method \"$ipv4_method\""
    fi

    # Get the ipv4.addresses
    ipv4_address=$(nmcli -g ipv4.addresses connection show "$unconnected_connection")
    if [ "$ipv4_address" != "--" ]; then
        nmcli_cmd+=" ipv4.addresses \"$ipv4_address\""
    fi

    # Get the ipv4.gateway
    ipv4_gateway=$(nmcli -g ipv4.gateway connection show "$unconnected_connection")
    if [ "$ipv4_gateway" != "--" ]; then
        nmcli_cmd+=" ipv4.gateway \"$ipv4_gateway\""
    fi

    # Get the ipv4.dns
    ipv4_dns=$(nmcli -g ipv4.dns connection show "$unconnected_connection")
    if [ "$ipv4_dns" != "--" ]; then
        nmcli_cmd+=" ipv4.dns \"$ipv4_dns\""
    fi

    # Run the built nmcli command if it has modifications
    if [ "$nmcli_cmd" != "nmcli connection modify \"$connection\"" ]; then
        eval "$nmcli_cmd"
    fi

    echo "IPv4 settings copied successfully."
}

# Function to copy IPv6 fields
copy_ipv6_fields() {
    echo "Copying IPv6 settings from $unconnected_connection to $connection..."

    # Initialize the nmcli command with the connection name
    nmcli_cmd="nmcli connection modify \"$connection\""

    # Get the ipv6.method
    ipv6_method=$(nmcli -g ipv6.method connection show "$unconnected_connection")
    if [ "$ipv6_method" != "--" ]; then
        nmcli_cmd+=" ipv6.method \"$ipv6_method\""
    fi

    # Get the ipv6.addresses
    ipv6_address=$(nmcli -g ipv6.addresses connection show "$unconnected_connection")
    if [ "$ipv6_address" != "--" ]; then
        nmcli_cmd+=" ipv6.addresses \"$ipv6_address\""
    fi

    # Get the ipv6.gateway
    ipv6_gateway=$(nmcli -g ipv6.gateway connection show "$unconnected_connection")
    if [ "$ipv6_gateway" != "--" ]; then
        nmcli_cmd+=" ipv6.gateway \"$ipv6_gateway\""
    fi

    # Get the ipv6.dns
    ipv6_dns=$(nmcli -g ipv6.dns connection show "$unconnected_connection")
    if [ "$ipv6_dns" != "--" ]; then
        nmcli_cmd+=" ipv6.dns \"$ipv6_dns\""
    fi

    # Run the built nmcli command if it has modifications
    if [ "$nmcli_cmd" != "nmcli connection modify \"$connection\"" ]; then
        eval "$nmcli_cmd"
    fi

    echo "IPv6 settings copied successfully."
}

# Main script execution
main() {
    check_tools
    identify_device
    identify_connection
    find_unconnected_connection
    copy_ipv4_fields
    copy_ipv6_fields

    # Restart connection
    nmcli connection down "$connection"
    nmcli connection up "$connection"
}

# Run the main function
main
`
