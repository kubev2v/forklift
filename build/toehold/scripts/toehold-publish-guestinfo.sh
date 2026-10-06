#!/bin/bash
# Push guest network info to vCenter once a routable address is configured.
set -euo pipefail

stamp=/var/lib/toehold/guestinfo-published
root=/opt/toehold/appliance-root
toolbox="${root}/usr/bin/vmware-toolbox-cmd"

[[ -f "${stamp}" ]] && exit 0

has_routable_ip() {
	local addr
	while read -r addr; do
		case "${addr}" in
		127.* | 169.254.* | "") continue ;;
		*) return 0 ;;
		esac
	done < <(ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1)
	return 1
}

if ! has_routable_ip; then
	exit 0
fi

if [[ -x "${toolbox}" ]]; then
	env LD_LIBRARY_PATH="${root}/usr/lib64:${root}/usr/lib" \
		"${toolbox}" info update network || true
fi

systemctl try-restart toehold-vmtoolsd.service || true
mkdir -p "$(dirname "${stamp}")"
touch "${stamp}"
exit 0
