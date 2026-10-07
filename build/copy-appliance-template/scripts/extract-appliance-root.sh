#!/bin/bash
set -eux

tar -xzf /root/appliance-root.tar.gz -C /
rm -f /root/appliance-root.tar.gz

want=/etc/systemd/system/multi-user.target.wants
mkdir -p "${want}"
if [[ -f /etc/systemd/system/sshd.service || -f /usr/lib/systemd/system/sshd.service ]]; then
  ln -sfn ../sshd.service "${want}/sshd.service"
fi

command -v restorecon >/dev/null 2>&1 && restorecon -R /opt/copy-appliance-template/appliance-root || true
