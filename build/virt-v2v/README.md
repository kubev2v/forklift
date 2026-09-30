# virt-v2v container image

Community image: [`Containerfile`](Containerfile) (EL10, UBI10 + CentOS Stream 10 repos).

Red Hat product build: [`Containerfile-downstream`](Containerfile-downstream).

## Legacy virtio-win (ISO in the image)

| Stage | What |
|-------|------|
| **Runtime** | `/usr/local/virtio-win-legacy.iso` (virt-v2v `--virtio-win-legacy-drivers`) |
| **Downstream** | `virtio-win-1.9.12-4.el7` RPM → `/usr/share/virtio-win/virtio-win.iso` |
| **Community** | [Fedora People](https://fedorapeople.org/groups/virt/virtio-win/repo/rpms/) `virtio-win-0.1.160-1.noarch.rpm` |

Prefetch (required; the winlegacyiso stage installs the prefetched RPM and the build fails without it):

```bash
make fetch-virtio-win-legacy fetch-kernel-modules-internal
```

Btrfs guests need `btrfs.ko` and the userspace `btrfs` tool inside the libguestfs appliance. libguestfs builds the appliance at runtime with supermin from the skeleton in `/usr/lib64/guestfs/supermin.d`, and that build copies the kernel modules from `/lib/modules/<kver>/kernel/`. CS10, however, ships `btrfs.ko` under `/lib/modules/<kver>/internal/`, so the runtime stage installs the prefetched `kernel-modules-internal-6.12.0-271.el10.x86_64.rpm` (whose kernel deps resolve from CS10) and moves `btrfs.ko` next to the other filesystem modules. The build fails if that RPM is missing. The runtime stage also installs `btrfs-progs` and appends that package name to the supermin skeleton `packages` file so supermin copies the tools and their RPM dependencies (including `libgcrypt`) into the appliance. `list_filesystems` runs `btrfs subvolume list` there to map subvolumes (SUSE uses btrfs with `@` and `boot` subvolumes).

Overrides:

```bash
VIRTIO_WIN_LEGACY_RPM_PATH=/path/to.rpm make fetch-virtio-win-legacy
VIRTIO_WIN_LEGACY_RPM_BASENAME=virtio-win-0.1.285-1.noarch.rpm \
VIRTIO_WIN_LEGACY_RPM_URL=https://fedorapeople.org/groups/virt/virtio-win/repo/rpms/virtio-win-0.1.285-1.noarch.rpm \
  make fetch-virtio-win-legacy
```

## Operator integration

- Default virt-v2v image: `VIRT_V2V_IMAGE`
- Alternate image slot (`VIRT_V2V_IMAGE_XFS` / bundle `VIRT_V2V_IMAGE_RHEL9`): same community image as `VIRT_V2V_IMAGE` for upstream bundles
