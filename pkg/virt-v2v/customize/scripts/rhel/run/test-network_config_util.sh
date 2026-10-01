#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

PASS() { echo "PASS: $@" ; }
PASS() { ${PASS_IS_FATAL:-false} && { echo "PASS (unexpected): $@" >&2 ; exit 1 ; } || { echo "PASS: $@" ; } ; }
FAIL() { ${FAIL_IS_FATAL:-true} && { echo "FAIL: $@" >&2 ; exit 1 ; } || { echo "FAIL (known): $@" ; } ; }

header() { echo -e "\n#\n# $@\n#\n" ; }
show_file() { echo "// File '$1'" ; cat "$1" | sed "s/^/| /" ; echo "\\\\" ; }

test_dir() {
    export TEST_DIR=$(mktemp -d --suffix="-forklift")

    # Paths for the test
    export V2V_MAP_FILE="$TEST_DIR/tmp/macToIP"
    export NETWORK_SCRIPTS_DIR="$TEST_DIR/etc/sysconfig/network-scripts"
    export NETWORK_SCRIPTS_DIR_SUSE="$TEST_DIR/etc/sysconfig/network"
    export NETWORK_CONNECTIONS_DIR="$TEST_DIR/etc/NetworkManager/system-connections"
    export UDEV_RULES_FILE="$TEST_DIR/etc/udev/rules.d/70-persistent-net.rules"
    export SYSTEMD_LINK_DIR="$TEST_DIR/etc/systemd/network"
    export SYSTEMD_NETWORK_DIR="$TEST_DIR/run/systemd/network"
    export NETPLAN_DIR="$TEST_DIR/"
    export NM_LEASES_DIR="$TEST_DIR/var/lib/NetworkManager"
    export DHCLIENT_LEASES_DIR="$TEST_DIR/var/lib/dhclient"
    export WICKED_DIR="$TEST_DIR/var/lib/wicked"

    export IFQUERY_CMD="
      podman run
      -v $TEST_DIR/etc/network:/etc/network
      quay.io/kubev2v/ifquery:latest ifquery
    "
    export TEST_SRC_DIR=$1  #${SCRIPT_DIR}/ifcfg-test.d
    export EXPECTED_UDEV_RULE_FILE="$TEST_SRC_DIR/expected-udev.rule"

    export IN_TESTING=true
    export PATH=$PATH:"$TEST_DIR/bin"

    header "Testing: $(basename $TEST_SRC_DIR)"

    cp -a $TEST_SRC_DIR/root/* $TEST_DIR

    # Fix netplan config file permissions (netplan requires 600)
    if [ -f "$TEST_DIR/etc/netplan/50-netplan.yaml" ]; then
        chmod 600 "$TEST_DIR/etc/netplan/50-netplan.yaml"
    fi

    # Clean up from previous runs
    rm -f "$UDEV_RULES_FILE"
    mkdir -p $(dirname "$UDEV_RULES_FILE")

    # Source the script under test
    {
    . ${SCRIPT_DIR}/network_config_util.sh
    } > $TEST_DIR/main.log 2>&1

    # Test 1: Verify the udev rules file was created
    if [ ! -f "$UDEV_RULES_FILE" ]; then
        [ "$FAIL_IS_FATAL" = "true" ] && show_file $TEST_DIR/main.log
        FAIL "UDEV_RULES_FILE not created."
    fi

    if ! cmp -s $EXPECTED_UDEV_RULE_FILE $UDEV_RULES_FILE ; then
        [ "$FAIL_IS_FATAL" = "true" ] && {
            show_file $UDEV_RULES_FILE
            diff -u $EXPECTED_UDEV_RULE_FILE $UDEV_RULES_FILE
            show_file $TEST_DIR/main.log
        }
        FAIL "The content of $UDEV_RULES_FILE does not match the expected rule."
    fi

    # Verify .link files were generated if expected
    if [ -d "$TEST_SRC_DIR/expected-link" ]; then
        for EXPECTED_LINK in "$TEST_SRC_DIR/expected-link"/*; do
            LINK_BASENAME=$(basename "$EXPECTED_LINK")
            ACTUAL_LINK="$SYSTEMD_LINK_DIR/$LINK_BASENAME"
            if [ ! -f "$ACTUAL_LINK" ]; then
                [ "$FAIL_IS_FATAL" = "true" ] && show_file $TEST_DIR/main.log
                FAIL "Expected .link file $LINK_BASENAME not found."
            fi
            if ! cmp -s "$EXPECTED_LINK" "$ACTUAL_LINK"; then
                [ "$FAIL_IS_FATAL" = "true" ] && {
                    show_file "$ACTUAL_LINK"
                    diff -u "$EXPECTED_LINK" "$ACTUAL_LINK"
                    show_file $TEST_DIR/main.log
                }
                FAIL ".link file $LINK_BASENAME does not match expected content."
            fi
        done
    fi

    PASS_IS_FATAL=false PASS $(basename $TEST_SRC_DIR)
    rm -rf "$TEST_DIR"
}

test_dirs() {
    for THE_DIR in $@;
    do test_dir "$THE_DIR";
    done
}

expected_to_pass_dirs() {
    local FAIL_IS_FATAL=true PASS_IS_FATAL=false
    test_dirs "$@"
}


expected_to_fail_dirs() {
    local FAIL_IS_FATAL=true PASS_IS_FATAL=true
    test_dirs "$@"
}


# Test systems using network-scripts
# ----------------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/ifcfg-*-test.d;
expected_to_fail_dirs ${SCRIPT_DIR}/ifcfg-*-test-failure.d;

# Test systems using system-connections
# -------------------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/networkmanager*-test.d;

# Test systems using netplan YAML
# -------------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/netplan*-test.d;

# Pod network IPv6 fix (no macToIP required)
# -----------------------------------------
header "Pod IPv6: patch only Pod MAC nmconnection"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-test")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/ns-absent"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/ns-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
mkdir -p "$NETWORK_CONNECTIONS_DIR"
echo "00:50:56:97:33:d8" > "$V2V_POD_NETWORK_MACS_FILE"
cat > "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" <<'EOF'
[connection]
id=eth0
type=ethernet
interface-name=eth0

[ipv4]
method=auto

[ipv6]
method=disabled
may-fail=false

[ethernet]
mac-address=00:50:56:97:33:D8
EOF
chmod 600 "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection"
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
if grep -q '^method=auto' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" \
    && grep -q '^may-fail=true' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" \
    && grep -q '^gateway=fd10:0:2::1' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection"; then
    PASS "Pod MAC nmconnection patched"
else
    FAIL "Pod MAC nmconnection NOT patched"
fi
rm -rf "$POD_IPV6_DIR"

header "Pod IPv6: skip non-Pod MAC nmconnection"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-test")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/ns-absent"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/ns-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
mkdir -p "$NETWORK_CONNECTIONS_DIR"
echo "00:50:56:97:33:d8" > "$V2V_POD_NETWORK_MACS_FILE"
cat > "$NETWORK_CONNECTIONS_DIR/other.nmconnection" <<'EOF'
[connection]
id=other
type=ethernet

[ipv6]
method=disabled
may-fail=false

[ethernet]
mac-address=00:11:22:33:44:55
EOF
chmod 600 "$NETWORK_CONNECTIONS_DIR/other.nmconnection"
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
if grep -q '^method=disabled' "$NETWORK_CONNECTIONS_DIR/other.nmconnection"; then
    PASS "non-Pod nmconnection left unchanged"
else
    FAIL "non-Pod nmconnection was modified"
fi
rm -rf "$POD_IPV6_DIR"

header "Pod IPv6: create ifcfg when ifcfg dir exists but no matching file (RHEL8)"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-ifcfg-test")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/network-scripts"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/network-scripts-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
# Create the ifcfg directory (empty) — simulates RHEL8 with no ifcfg for this NIC.
mkdir -p "$NETWORK_SCRIPTS_DIR"
echo "00:50:56:97:33:d8" > "$V2V_POD_NETWORK_MACS_FILE"
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
CREATED_FILE="$NETWORK_SCRIPTS_DIR/ifcfg-pod-00-50-56-97-33-d8"
if [ -f "$CREATED_FILE" ] \
    && grep -q '^HWADDR=00:50:56:97:33:d8' "$CREATED_FILE" \
    && grep -q '^BOOTPROTO=dhcp' "$CREATED_FILE" \
    && grep -q '^ONBOOT=yes' "$CREATED_FILE" \
    && grep -q '^IPV6INIT=yes' "$CREATED_FILE" \
    && grep -q '^IPV6_DEFAULTGW=fd10:0:2::1' "$CREATED_FILE"; then
    PASS "Pod ifcfg created with correct settings (RHEL8)"
else
    echo "--- $CREATED_FILE ---"
    cat "$CREATED_FILE" 2>/dev/null || echo "(file not found)"
    FAIL "Pod ifcfg NOT created or missing settings"
fi
rm -rf "$POD_IPV6_DIR"

header "Pod IPv6: create keyfile when no ifcfg dir exists (RHEL9+)"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-keyfile-test")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/network-scripts-absent"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/network-scripts-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
echo "00:50:56:97:33:d8" > "$V2V_POD_NETWORK_MACS_FILE"
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
CREATED_FILE="$NETWORK_CONNECTIONS_DIR/pod-00-50-56-97-33-d8.nmconnection"
if [ -f "$CREATED_FILE" ] \
    && grep -q '^mac-address=00:50:56:97:33:d8' "$CREATED_FILE" \
    && grep -q '^method=auto' "$CREATED_FILE" \
    && grep -q '^may-fail=true' "$CREATED_FILE" \
    && grep -q '^gateway=fd10:0:2::1' "$CREATED_FILE" \
    && grep -q '^autoconnect=true' "$CREATED_FILE"; then
    PASS "Pod keyfile created with correct settings (RHEL9+)"
else
    echo "--- $CREATED_FILE ---"
    cat "$CREATED_FILE" 2>/dev/null || echo "(file not found)"
    FAIL "Pod keyfile NOT created or missing settings"
fi
rm -rf "$POD_IPV6_DIR"

header "Pod IPv6: quoted HWADDR ifcfg is matched and patched"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-quoted-hwaddr")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/network-scripts"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/network-scripts-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
mkdir -p "$NETWORK_SCRIPTS_DIR"
echo "00:50:56:97:33:d8" > "$V2V_POD_NETWORK_MACS_FILE"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-ens3" <<'EOF'
TYPE=Ethernet
BOOTPROTO=dhcp
ONBOOT=yes
HWADDR="00:50:56:97:33:D8"
DEVICE=ens3
EOF
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
if grep -q '^IPV6_DEFAULTGW=fd10:0:2::1' "$NETWORK_SCRIPTS_DIR/ifcfg-ens3" \
    && grep -q '^IPV6INIT=yes' "$NETWORK_SCRIPTS_DIR/ifcfg-ens3" \
    && grep -q '^IPV4_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-ens3"; then
    PASS "quoted HWADDR ifcfg matched and patched"
else
    echo "--- ifcfg-ens3 ---"
    cat "$NETWORK_SCRIPTS_DIR/ifcfg-ens3" 2>/dev/null || echo "(file not found)"
    FAIL "quoted HWADDR ifcfg NOT matched/patched"
fi
rm -rf "$POD_IPV6_DIR"

header "Pod IPv6: created ifcfg omits DEVICE when udev name is unknown"
POD_IPV6_DIR=$(mktemp -d --suffix="-pod-ipv6-no-device")
export V2V_POD_NETWORK_MACS_FILE="$POD_IPV6_DIR/podNetworkMACs"
export NETWORK_CONNECTIONS_DIR="$POD_IPV6_DIR/system-connections"
export NETWORK_SCRIPTS_DIR="$POD_IPV6_DIR/network-scripts"
export NETWORK_SCRIPTS_DIR_SUSE="$POD_IPV6_DIR/network-scripts-suse-absent"
export UDEV_RULES_FILE="$POD_IPV6_DIR/70-persistent-net.rules"
export V2V_MAP_FILE="$POD_IPV6_DIR/no-macToIP"
mkdir -p "$NETWORK_SCRIPTS_DIR"
# Use a MAC with no udev rule to confirm DEVICE is omitted.
echo "aa:bb:cc:dd:ee:ff" > "$V2V_POD_NETWORK_MACS_FILE"
( . "${SCRIPT_DIR}/network_config_util.sh" ) >/dev/null 2>&1 || true
CREATED_FILE="$NETWORK_SCRIPTS_DIR/ifcfg-pod-aa-bb-cc-dd-ee-ff"
if [ -f "$CREATED_FILE" ] \
    && grep -q '^HWADDR=aa:bb:cc:dd:ee:ff' "$CREATED_FILE" \
    && ! grep -q '^DEVICE=' "$CREATED_FILE"; then
    PASS "created ifcfg omits DEVICE when udev name unknown"
else
    echo "--- $CREATED_FILE ---"
    cat "$CREATED_FILE" 2>/dev/null || echo "(file not found)"
    FAIL "created ifcfg should not contain DEVICE"
fi
rm -rf "$POD_IPV6_DIR"

PASS "All Pod and pass/fail tests behaved as expected."

# Test systems using systemd (expected to fail — pre-existing)
# --------------------------
DISABLE_NETPLAN_GET=true expected_to_fail_dirs ${SCRIPT_DIR}/systemd*-test.d;

# Test systems using network interfaces
# --------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/network-interfaces*-test.d;

# Test systems using dhcp
# --------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/dhcp*-test.d;

PASS "All tests behaved as expected."
