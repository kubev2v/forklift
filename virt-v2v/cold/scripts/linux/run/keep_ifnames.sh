#!/bin/bash

# Global variables with default values
NETWORK_SCRIPTS_DIR="${NETWORK_SCRIPTS_DIR:-/etc/sysconfig/network-scripts}"
NETWORK_CONNECTIONS_DIR="${NETWORK_CONNECTIONS_DIR:-/etc/NetworkManager/system-connections}"
UDEV_RULES_FILE="${UDEV_RULES_FILE:-/etc/udev/rules.d/70-persistent-net.rules}"

ret_with() { echo "Return: $@" ; }
exit_err_with() { echo "ERR Exit: $@" ; exit 1 ; }

# Check if udev rules file exists
if [ -f "$UDEV_RULES_FILE" ]; then
    exit_err_with "File $UDEV_RULES_FILE already exists. Exiting."
fi

# _udev_rule HWADDR IFNAME
:> /var/tmp/70.stage
stage_udev_rule() {
    local HWADDR="$1"
    local IFNAME="$2"
    echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$HWADDR\",NAME=\"$IFNAME\"" >> /var/tmp/70.stage
}
cat_staged_udev_rules() { cat /var/tmp/70.stage ; }
write_udev_rules() { 
    echo "Old rules:"
    cat $UDEV_RULES_FILE
    echo "New rules"
    cat /var/tmp/70.stage
    mv -v /var/tmp/70.stage $UDEV_RULES_FILE ;
}

# FN HWADDR IFNAME
try_stage_udev_rule() {
    local FN="$1"
    local FNKEY="$2"
    local HWADDR="$3"
    local IFNAME="$4"

    [ -z "$HWADDR" ] && { echo "'$FNKEY' not set in '$FN', unable to match against an interface. Continuing" ; return 1 ; }
    [ -n "$IFNAME" ] && stage_udev_rule $HWADDR $IFNAME
}

# Create udev rules based on the macToip mapping + ifcfg network scripts
udev_from_ifcfg() {
    # Check if the network scripts directory exists
    [ -d "$NETWORK_SCRIPTS_DIR" ] || ret_with "Warning: Directory $NETWORK_SCRIPTS_DIR does not exist."

    for IFCFG_FN in $NETWORK_SCRIPTS_DIR/*;
    do
        FN_PARSED_IFNAME=${IFCFG_FN#$NETWORK_SCRIPTS_DIR/ifcfg-}
        source $IFCFG_FN
        
        IFNAME=${DEVICE:-$FN_PARSED_IFNAME}
        try_stage_udev_rule "$IFCFG_FN" "HWADDR" "$HWADDR" "$IFNAME"
    done
}

iniToEnv() {
    _sani() { tr -d "[]" | tr "-" "_" ; }
    local SECT=""
    while read LINE ;
    do
        [ -z "$LINE" ] && continue ; 
        SANI_LINE=$(echo "$LINE" | _sani) ;
        echo "$LINE" | grep -q -E "^\[" && { SECT=$SANI_LINE ; continue ; }
        echo "${SECT}__${SANI_LINE}"
    done
}
# Create udev rules based on the macToip mapping + network manager connections
udev_from_nm() {
    # Check if the network connections directory exists
    [ ! -d "$NETWORK_CONNECTIONS_DIR" ] || ret_with "Warning: Directory $NETWORK_CONNECTIONS_DIR does not exist."

    for NM_FN in $NETWORK_CONNECTIONS_DIR/*;
    do
        eval $(cat $NM_FN | iniToEnv)
        try_stage_udev_rule "$NM_FN" "ethernet.mac-address" "$ethernet__mac_address" "$connection__interface_name"
    done
}

# Create udev rules check for duplicates and write them to udev file
main() {
    udev_from_ifcfg
    udev_from_nm
    write_udev_rules
}

main
