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

# Test fix_nm_ipv6_may_fail() in isolation
# -----------------------------------------
# These tests source network_config_util.sh to get the function, then verify
# it correctly patches NM keyfiles and ifcfg scripts for IPv6 may-fail.

IPV6_TEST_DIR=""

setup_ipv6_test() {
    IPV6_TEST_DIR=$(mktemp -d --suffix="-ipv6-test")
    export NETWORK_CONNECTIONS_DIR="$IPV6_TEST_DIR/system-connections"
    export NETWORK_SCRIPTS_DIR="$IPV6_TEST_DIR/network-scripts"
    export NETWORK_SCRIPTS_DIR_SUSE="$IPV6_TEST_DIR/suse-network"
    export V2V_MAP_FILE="$IPV6_TEST_DIR/macToIP"
}

cleanup_ipv6_test() {
    rm -rf "$IPV6_TEST_DIR"
}

run_ipv6_fix() {
    # Source the script in a subshell to get fix_nm_ipv6_may_fail, then
    # exit before it reaches the macToIP check (which would exit 0).
    ( . "${SCRIPT_DIR}/network_config_util.sh" ) 2>/dev/null || true
}

# --- NM keyfile tests ---

header "IPv6: NM keyfile may-fail=false → true"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" <<'EOF'
[connection]
id=eth0
type=ethernet

[ipv6]
method=auto
may-fail=false
EOF
run_ipv6_fix
if grep -q '^may-fail=true' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection"; then
    PASS "NM keyfile may-fail=false changed to true"
else
    FAIL "NM keyfile may-fail=false NOT changed"
fi
cleanup_ipv6_test

header "IPv6: NM keyfile may-fail absent → added"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" <<'EOF'
[connection]
id=eth0
type=ethernet

[ipv6]
method=auto
EOF
run_ipv6_fix
if grep -q '^may-fail=true' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection"; then
    PASS "NM keyfile may-fail added when absent"
else
    FAIL "NM keyfile may-fail NOT added when absent"
fi
cleanup_ipv6_test

header "IPv6: NM keyfile may-fail=true unchanged"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" <<'EOF'
[connection]
id=eth0

[ipv6]
method=auto
may-fail=true
EOF
run_ipv6_fix
COUNT=$(grep -c '^may-fail=true' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection")
if [ "$COUNT" -eq 1 ]; then
    PASS "NM keyfile may-fail=true left unchanged"
else
    FAIL "NM keyfile may-fail=true duplicated or missing (count=$COUNT)"
fi
cleanup_ipv6_test

header "IPv6: NM keyfile method=auto with trailing space"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection" <<'EOF'
[connection]
id=eth0

[ipv6]
method=auto 
may-fail=false
EOF
run_ipv6_fix
if grep -q '^may-fail=true' "$NETWORK_CONNECTIONS_DIR/eth0.nmconnection"; then
    PASS "NM keyfile method=auto (trailing space) fixed"
else
    FAIL "NM keyfile method=auto (trailing space) NOT fixed (silently skipped)"
fi
cleanup_ipv6_test

header "IPv6: NM keyfile method=manual skipped"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/static.nmconnection" <<'EOF'
[connection]
id=static

[ipv6]
method=manual
may-fail=false
EOF
run_ipv6_fix
if grep -q '^may-fail=false' "$NETWORK_CONNECTIONS_DIR/static.nmconnection"; then
    PASS "NM keyfile method=manual not touched"
else
    FAIL "NM keyfile method=manual was incorrectly modified"
fi
cleanup_ipv6_test

header "IPv6: NM keyfile no [ipv6] section skipped"
setup_ipv6_test
mkdir -p "$NETWORK_CONNECTIONS_DIR"
cat > "$NETWORK_CONNECTIONS_DIR/v4only.nmconnection" <<'EOF'
[connection]
id=v4only

[ipv4]
method=auto
EOF
run_ipv6_fix
if ! grep -q 'may-fail' "$NETWORK_CONNECTIONS_DIR/v4only.nmconnection"; then
    PASS "NM keyfile without [ipv6] not touched"
else
    FAIL "NM keyfile without [ipv6] was incorrectly modified"
fi
cleanup_ipv6_test

# --- ifcfg tests ---

header "IPv6: ifcfg IPV6_FAILURE_FATAL=yes → no"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-eth0" <<'EOF'
DEVICE=eth0
BOOTPROTO=static
IPV6INIT=yes
IPV6_AUTOCONF=yes
IPV6_FAILURE_FATAL=yes
EOF
run_ipv6_fix
if grep -q '^IPV6_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-eth0"; then
    PASS "ifcfg IPV6_FAILURE_FATAL=yes changed to no"
else
    FAIL "ifcfg IPV6_FAILURE_FATAL=yes NOT changed"
fi
cleanup_ipv6_test

header "IPv6: ifcfg IPV6_FAILURE_FATAL absent → added"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-eth0" <<'EOF'
DEVICE=eth0
BOOTPROTO=static
IPV6INIT=yes
IPV6_AUTOCONF=yes
EOF
run_ipv6_fix
if grep -q '^IPV6_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-eth0"; then
    PASS "ifcfg IPV6_FAILURE_FATAL=no added when absent"
else
    FAIL "ifcfg IPV6_FAILURE_FATAL=no NOT added when absent"
fi
cleanup_ipv6_test

header "IPv6: ifcfg IPV6_FAILURE_FATAL=no unchanged"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-eth0" <<'EOF'
DEVICE=eth0
IPV6INIT=yes
IPV6_FAILURE_FATAL=no
EOF
run_ipv6_fix
COUNT=$(grep -c '^IPV6_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-eth0")
if [ "$COUNT" -eq 1 ]; then
    PASS "ifcfg IPV6_FAILURE_FATAL=no left unchanged"
else
    FAIL "ifcfg IPV6_FAILURE_FATAL=no duplicated or missing (count=$COUNT)"
fi
cleanup_ipv6_test

header "IPv6: ifcfg-lo skipped"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-lo" <<'EOF'
DEVICE=lo
IPV6INIT=yes
IPV6_FAILURE_FATAL=yes
EOF
run_ipv6_fix
if grep -q '^IPV6_FAILURE_FATAL=yes' "$NETWORK_SCRIPTS_DIR/ifcfg-lo"; then
    PASS "ifcfg-lo not touched"
else
    FAIL "ifcfg-lo was incorrectly modified"
fi
cleanup_ipv6_test

header "IPv6: ifcfg without IPV6INIT skipped"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-eth0" <<'EOF'
DEVICE=eth0
BOOTPROTO=dhcp
IPV6_FAILURE_FATAL=yes
EOF
run_ipv6_fix
if grep -q '^IPV6_FAILURE_FATAL=yes' "$NETWORK_SCRIPTS_DIR/ifcfg-eth0"; then
    PASS "ifcfg without IPV6INIT not touched"
else
    FAIL "ifcfg without IPV6INIT was incorrectly modified"
fi
cleanup_ipv6_test

header "IPv6: ifcfg whitespace around = handled"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-eth0" <<'EOF'
DEVICE=eth0
IPV6INIT = yes
IPV6_FAILURE_FATAL = yes
EOF
run_ipv6_fix
if grep -q 'IPV6_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-eth0"; then
    PASS "ifcfg whitespace around = handled"
else
    FAIL "ifcfg whitespace around = NOT handled"
fi
cleanup_ipv6_test

header "IPv6: static IPv4 + IPv6 SLAAC (no BOOTPROTO=dhcp)"
setup_ipv6_test
mkdir -p "$NETWORK_SCRIPTS_DIR"
cat > "$NETWORK_SCRIPTS_DIR/ifcfg-ens192" <<'EOF'
DEVICE=ens192
BOOTPROTO=none
IPADDR=10.0.0.5
PREFIX=24
IPV6INIT=yes
IPV6_AUTOCONF=yes
IPV6_FAILURE_FATAL=yes
EOF
run_ipv6_fix
if grep -q '^IPV6_FAILURE_FATAL=no' "$NETWORK_SCRIPTS_DIR/ifcfg-ens192"; then
    PASS "static IPv4 + IPv6 SLAAC: IPV6_FAILURE_FATAL fixed"
else
    FAIL "static IPv4 + IPv6 SLAAC: IPV6_FAILURE_FATAL NOT fixed"
fi
cleanup_ipv6_test


# Test systems using systemd
# --------------------------
DISABLE_NETPLAN_GET=true expected_to_fail_dirs ${SCRIPT_DIR}/systemd*-test.d;

# Test systems using network interfaces
# --------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/network-interfaces*-test.d;

# Test systems using dhcp
# --------------------------
expected_to_pass_dirs ${SCRIPT_DIR}/dhcp*-test.d;

PASS "All tests behaved as expected."
