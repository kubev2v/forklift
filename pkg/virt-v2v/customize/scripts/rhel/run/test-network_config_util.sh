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
    export SYSTEMD_NETWORK_DIR="$TEST_DIR/run/systemd/network"
    export NETPLAN_DIR="$TEST_DIR/"
    export NM_LEASES_DIR="$TEST_DIR/var/lib/NetworkManager"
    export DHCLIENT_LEASES_DIR="$TEST_DIR/var/lib/dhclient"

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
