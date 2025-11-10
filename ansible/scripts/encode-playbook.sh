#!/bin/bash
# Encode an Ansible playbook to base64 for use in MTV Hook CRs
#
# Usage: ./encode-playbook.sh <playbook.yml>
#
# Example:
#   ./encode-playbook.sh ../prehook-cloud-init/playbook.yml

set -e

if [ $# -eq 0 ]; then
    echo "Error: No playbook file specified"
    echo ""
    echo "Usage: $0 <playbook.yml>"
    echo ""
    echo "Example:"
    echo "  $0 ../prehook-cloud-init/playbook.yml"
    exit 1
fi

PLAYBOOK_FILE="$1"

if [ ! -f "$PLAYBOOK_FILE" ]; then
    echo "Error: File '$PLAYBOOK_FILE' not found"
    exit 1
fi

echo "Encoding playbook: $PLAYBOOK_FILE"
echo ""
echo "Base64 encoded playbook (copy this into your Hook CR spec.playbook field):"
echo "---"
cat "$PLAYBOOK_FILE" | base64 -w0 || cat "$PLAYBOOK_FILE" | base64
echo ""
echo "---"
echo ""
echo "To decode and verify:"
echo "  echo '<encoded-string>' | base64 -d"

