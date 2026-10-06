#!/usr/bin/env bash
# Build-time: install CentOS Stream 9 tools into a tarball that virt-customize
# unpacks onto the toehold base image (see extract-appliance-root).
set -euxo pipefail

staging=/tmp/appliance-staging
root="${staging}/opt/toehold/appliance-root"
tarball=/usr/share/toehold/appliance-root.tar.gz

rm -rf "${staging}"
mkdir -p "${root}" "$(dirname "${tarball}")"

dnf install -y \
  --installroot="${root}" \
  --releasever=9 \
  --nodocs \
  --nogpgcheck \
  --setopt=install_weak_deps=False \
  --repofrompath=cs9-baseos,"https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/" \
  --repofrompath=cs9-appstream,"https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/" \
  open-vm-tools \
  podman \
  containernetworking-plugins

# Installroot podman needs vfs (no kernel overlay in the guest appliance) and a
# policy path that survives under /opt/toehold/appliance-root.
conf="${root}/etc/containers"
sed -i 's/^driver = "overlay"/driver = "vfs"/' "${conf}/storage.conf"
cat > "${conf}/containers.conf" <<EOF
[engine]
signature_policy = "/opt/toehold/appliance-root/etc/containers/policy.json"
EOF
[[ -f "${root}/etc/vmware-tools/tools.conf" ]] || \
  cp "${root}/etc/vmware-tools/tools.conf.example" "${root}/etc/vmware-tools/tools.conf"

mkdir -p \
  "${staging}/etc/containers" \
  "${staging}/etc/systemd/system/multi-user.target.wants" \
  "${staging}/etc/systemd/system/timers.target.wants" \
  "${staging}/etc/NetworkManager/system-connections" \
  "${staging}/usr/local/bin" \
  "${staging}/usr/bin"

cp /usr/share/toehold/systemd/*.service /usr/share/toehold/systemd/*.timer \
  "${staging}/etc/systemd/system/"
install -m755 \
  /usr/local/bin/toehold-publish-guestinfo.sh \
  /usr/local/bin/toehold-podman.sh \
  "${staging}/usr/local/bin/"
ln -sfn toehold-podman.sh "${staging}/usr/local/bin/toehold-podman"
ln -sfn /usr/local/bin/toehold-podman.sh "${staging}/usr/local/bin/podman"
ln -sfn /usr/local/bin/toehold-podman.sh "${staging}/usr/bin/podman"

# Host /etc sees the same configs the installroot binaries use.
for f in policy.json storage.conf registries.conf; do
  ln -sfn /opt/toehold/appliance-root/etc/containers/${f} \
    "${staging}/etc/containers/${f}"
done
ln -sfn /opt/toehold/appliance-root/etc/vmware-tools "${staging}/etc/vmware-tools"

want="${staging}/etc/systemd/system/multi-user.target.wants"
ln -sfn ../toehold-vgauthd.service "${want}/toehold-vgauthd.service"
ln -sfn ../toehold-vmtoolsd.service "${want}/toehold-vmtoolsd.service"
ln -sfn ../toehold-guestinfo-sync.timer \
  "${staging}/etc/systemd/system/timers.target.wants/toehold-guestinfo-sync.timer"

cat > "${staging}/etc/NetworkManager/system-connections/vsphere-dhcp.nmconnection" <<'EOF'
[connection]
id=vsphere-dhcp
type=ethernet
autoconnect=true
autoconnect-priority=100

[ethernet]

[ipv4]
method=auto

[ipv6]
method=ignore
EOF
chmod 600 "${staging}/etc/NetworkManager/system-connections/vsphere-dhcp.nmconnection"

tar -czf "${tarball}" -C "${staging}" .
