#!/bin/bash

# Global variables with default values
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"

ret_with() { echo "Return: $@" ; }
exit_err_with() { echo "ERR Exit: $@" ; exit 1 ; }

# Check if udev rules file exists
if [ -f "$UDEV_RULES_FILE" ]; then
    exit_with "File $UDEV_RULES_FILE already exists. Exiting."
fi

# _udev_rule HWADDR IFNAME
:> /var/tmp/70.stage
stage_udev_rule() { echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$1\",NAME=\"$2\"" >> /var/tmp/70.stage ; }
cat_staged_udev_rules() { cat /var/tmp/70.stage ; }
write_udev_rules() { 
    echo "Old rules:"
    cat $UDEV_RULES_FILE
    echo "New rules"
    cat /var/tmp/70.stage
    mv -v /var/tmp/70.stage $UDEV_RULES_FILE ;
}

# Create udev rules based on the macToip mapping + ifcfg network scripts
udev_from_ifcfg() {
    # Check if the network scripts directory exists
    [ -d "$NETWORK_SCRIPTS_DIR" ] || ret_with "Warning: Directory $NETWORK_SCRIPTS_DIR does not exist."

    for IFCFG_FN in $NETWORK_SCRIPTS_DIR/*;
    do
        FN_PARSED_IFNAME=${IFCFG_FN/*ifcfg-/}
        source $IFCFG_FN
        
        [ -z "$HWADDR" ] && { echo "HWADDR not set for $IFCFG_FN, unable to match, continuing" ; continue ; }

        IFNAME=${DEVICE:-$FN_PARSED_IFNAME}
        [ -n "$IFNAME" ] && _udev_rule $HWADDR $IFNAME
    done
}

# Create udev rules based on the macToip mapping + network manager connections
udev_from_nm() {
    # Check if the network connections directory exists
    if [ ! -d "$NETWORK_CONNECTIONS_DIR" ]; then
        echo "Warning: Directory $NETWORK_CONNECTIONS_DIR does not exist."
        return 0
    fi

   ret_with "NM: Unsupported right now" 
}

# Create udev rules check for duplicates and write them to udev file
main() {
    udev_from_ifcfg
    udev_from_nm
    write_udev_rules
}

main
