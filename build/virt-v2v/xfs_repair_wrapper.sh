#!/bin/sh
if grep -q 'xfs_repair_ignore=1' /proc/cmdline 2>/dev/null; then
	echo "xfs_repair wrapper: xfs_repair_ignore=1 detected, failures will be suppressed" >&2
	/usr/sbin/xfs_repair.original "$@"
	rc=$?
	echo "xfs_repair wrapper: xfs_repair.original exited with status $rc (ignored)" >&2
	exit 0
fi
exec /usr/sbin/xfs_repair.original "$@"
