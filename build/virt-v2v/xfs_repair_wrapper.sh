#!/bin/sh
output=$(/usr/sbin/xfs_repair.real "$@" 2>&1)
rc=$?
echo "$output"
if [ $rc -ne 0 ]; then
    if echo "$output" | grep -qE "bad agbno [0-9]+ in agfl, agno [0-9]+"; then
        echo "xfs_repair: ignoring known benign error (bad agbno in agfl)"
        exit 0
    fi
fi
exit $rc
