#!/bin/sh
#
# Start nbdkit in the foreground, exporting a host block device over plain NBD.
#
# Configuration (env vars, all optional):
#   DISK  path inside the container of the block device to export
#         (default: /dev/nbd-export -- map the host device here with
#          `podman run --device /dev/sdX:/dev/nbd-export:r`)
#   PORT  TCP port to listen on (default: 10809)

set -eu

DISK="${DISK:-/dev/nbd-export}"
PORT="${PORT:-10809}"

die() {
	echo "entrypoint: error: $*" >&2
	exit 1
}

# The export target must be a block device present in the container.
[ -e "$DISK" ] || die "device '$DISK' not found -- pass it in with '--device /dev/sdX:$DISK:r'"
[ -b "$DISK" ] || die "'$DISK' exists but is not a block device"

echo "entrypoint: exporting $DISK (read-only) on port $PORT"

# exec so nbdkit becomes PID 1 and receives signals directly.
exec nbdkit --foreground \
	--readonly \
	--port "$PORT" \
	file "$DISK"
