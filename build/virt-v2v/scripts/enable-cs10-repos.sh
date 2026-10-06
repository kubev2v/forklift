#!/usr/bin/env bash
# Enable a CentOS Stream 10 repo file on UBI (virt-v2v, libguestfs, kernel, etc.).
# Usage: enable-cs10-repos.sh <repo-file>
# <repo-file> is expected under /tmp/cs10-repos/ (e.g. centos-stream10-runtime.repo).
set -euo pipefail

if [ -z "${1:-}" ]; then
    echo "usage: $0 <repo-file under /tmp/cs10-repos/>" >&2
    exit 1
fi

install -d -m 0755 /etc/yum.repos.d
cp -f "/tmp/cs10-repos/${1}" "/etc/yum.repos.d/${1}"
