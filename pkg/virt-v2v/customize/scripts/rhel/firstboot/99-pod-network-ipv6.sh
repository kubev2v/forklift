#!/bin/bash
# Activate Pod-network NICs and ensure IPv6 connectivity on first boot (MTV-6872).
# Handles IPv4-only, dual-stack, and IPv6-only clusters.
#
# On IPv6-enabled clusters: assign the well-known masquerade guest IPv6
# address (required because the bridge suppresses Router Advertisements).
# On IPv4-only clusters: skip IPv6 config entirely to avoid unreachable
# routes that would cause applications to stall before falling back.
POD_MACS=/tmp/podNetworkIPv6MACs
GW6=fd10:0:2::1
GUEST_IP6=fd10:0:2::2
GUEST_CIDR6=120
MAX_ATTEMPTS=10

# This file only exists on IPv6-enabled clusters.
[ ! -s "$POD_MACS" ] && exit 0

exec >>/var/log/pod-network-ipv6-firstboot.log 2>&1
echo "=== $(date) ==="

for _ in $(seq 1 60); do
    nmcli general status >/dev/null 2>&1 && break
    sleep 2
done

normalize_mac() {
    echo "$1" | tr 'A-F' 'a-f' | tr -d '[:space:]'
}

connection_for_mac() {
    local want=$1 c pm
    while read -r c; do
        [ -z "$c" ] && continue
        pm=$(nmcli -g 802-3-ethernet.mac-address con show "$c" 2>/dev/null) || continue
        [ "$(normalize_mac "$pm")" = "$want" ] && { echo "$c"; return 0; }
    done < <(nmcli -t -f NAME con show 2>/dev/null)
    local dev
    dev=$(dev_for_mac "$want") || return 1
    while read -r c; do
        [ -z "$c" ] && continue
        local cdev
        cdev=$(nmcli -g GENERAL.DEVICES con show "$c" 2>/dev/null) || continue
        [ "$cdev" = "$dev" ] && { echo "$c"; return 0; }
    done < <(nmcli -t -f NAME con show 2>/dev/null)
    return 1
}

dev_for_mac() {
    local want=$1
    for dev_path in /sys/class/net/*; do
        [ -f "$dev_path/address" ] || continue
        local d
        d=$(basename "$dev_path")
        [ "$d" = "lo" ] && continue
        [ "$(normalize_mac "$(cat "$dev_path/address")")" = "$want" ] || continue
        echo "$d"
        return 0
    done
    return 1
}

while read -r mac; do
    want=$(normalize_mac "$mac")
    [ -z "$want" ] && continue
    echo "Processing Pod MAC: $want"

    dev=$(dev_for_mac "$want") || true
    if [ -z "$dev" ]; then
        echo "  WARNING: no device found for MAC $want"
        continue
    fi
    echo "  Device: $dev"

    nmcli device set "$dev" managed yes 2>/dev/null || true

    con=$(connection_for_mac "$want") || true
    echo "  Connection profile: ${con:-<none>}"

    if [ -n "$con" ]; then
        # Fix may-fail so DHCP timeout doesn't kill the connection.
        nmcli con modify "$con" ipv4.may-fail yes 2>/dev/null || true
        nmcli con modify "$con" ipv6.may-fail yes 2>/dev/null || true
        nmcli con modify "$con" connection.autoconnect yes 2>/dev/null || true
        nmcli con modify "$con" connection.master "" connection.slave-type "" 2>/dev/null || true

        # Static IPv6 — avoids waiting for SLAAC/DHCPv6 which may never
        # arrive (masquerade bridge has forwarding=1, suppresses RAs).
        nmcli con modify "$con" \
            ipv6.method manual \
            ipv6.addresses "${GUEST_IP6}/${GUEST_CIDR6}" \
            ipv6.gateway "$GW6" \
            2>/dev/null && echo "  Set static IPv6 ${GUEST_IP6}/${GUEST_CIDR6}" \
                        || echo "  WARNING: failed to set static IPv6"

        # Force NM to re-read the modified profile from disk.
        nmcli con reload 2>/dev/null || true

        # Retry activation — NM may need time to settle after boot,
        # especially on IPv6-only where IPv4 DHCP must time out first.
        activated=false
        for attempt in $(seq 1 $MAX_ATTEMPTS); do
            if nmcli con up "$con" ifname "$dev" 2>/dev/null; then
                echo "  Attempt $attempt: activated successfully"
                activated=true
                break
            fi
            echo "  Attempt $attempt: activation failed, waiting..."
            sleep 5
        done

        if ! $activated; then
            # Last-ditch: try without specifying ifname, then device connect.
            nmcli con up "$con" 2>/dev/null \
                || nmcli device connect "$dev" 2>/dev/null \
                || echo "  WARNING: all NM activation attempts failed"
        fi
    else
        nmcli device connect "$dev" 2>/dev/null || true
    fi

    # ensure IPv6 is configured regardless of NM.
    ip link set "$dev" up 2>/dev/null || true
    if ! ip -6 addr show dev "$dev" scope global 2>/dev/null | grep -q "$GUEST_IP6"; then
        ip -6 addr add "${GUEST_IP6}/${GUEST_CIDR6}" dev "$dev" 2>/dev/null || true
        echo "  Added $GUEST_IP6/$GUEST_CIDR6 manually"
    fi
    if ! ip -6 route show default dev "$dev" 2>/dev/null | grep -q "$GW6"; then
        ip -6 route add default via "$GW6" dev "$dev" 2>/dev/null || true
        echo "  Added default route via $GW6"
    fi

    echo "  Final state:"
    ip -4 addr show dev "$dev" 2>/dev/null | grep 'inet ' || echo "    No IPv4 (expected on IPv6-only)"
    ip -6 addr show dev "$dev" scope global 2>/dev/null | grep inet6 || echo "    No global IPv6"
    nmcli device status 2>/dev/null | grep "$dev" || true
    echo "  Done"
done < "$POD_MACS"

echo "=== Finished ==="
