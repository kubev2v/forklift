#!/bin/sh
# Tweak ESXi hostd-tmp memory limit for high-concurrency vmkfstools
# Usage: ./tweak-esxi-mem.sh <limit_in_mb>

if [ -z "$1" ]; then
    echo "Usage: $0 <limit_in_mb>"
    echo "Example: $0 1024"
    exit 1
fi

LIMIT_MB=$1

# Basic integer validation
case "$LIMIT_MB" in
    ''|*[!0-9]*) echo "Error: Memory limit must be a positive integer." >&2; exit 1 ;;
    *) ;;
esac

LIMIT_KB=$((LIMIT_MB * 1024))
echo "Setting hostd-tmp memory limit to ${LIMIT_MB} MB (${LIMIT_KB} KB)..."

# 1. Dynamically find the hostd-tmp group ID
GROUP_ID=""
for i in $(vsish -e ls /sched/groups); do
    name=$(vsish -e get /sched/groups/${i}groupName 2>/dev/null)
    if [ "$name" = "hostd-tmp" ]; then
        GROUP_ID=${i%/} # remove trailing slash
        break
    fi
done

if [ -z "$GROUP_ID" ]; then
    echo "Error: Could not find resource group 'hostd-tmp'."
    exit 1
fi

echo "Found hostd-tmp at group ID: $GROUP_ID"

# 2. Apply runtime change using vsish API
echo "Applying runtime memory limit..."
vsish -e set /sched/groups/${GROUP_ID}/memAllocation max=${LIMIT_KB}
if [ $? -eq 0 ]; then
    echo "✅ Runtime limit updated successfully."
else
    echo "❌ Failed to update runtime limit via vsish."
    exit 1
fi

# Verification
echo "--- Current Allocation ---"
vsish -e get /sched/groups/${GROUP_ID}/memAllocation | grep "max:"
echo "--------------------------"
echo "Note: This change is runtime-only and will not persist across reboots."
echo "Done."
