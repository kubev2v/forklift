# Guest Conversion

This document explains how EC2 instances are converted to run properly on KubeVirt/OpenShift Virtualization.

## Why Conversion Is Needed

EC2 instances use AWS-specific drivers and configurations that don't work in KubeVirt:

| EC2 Configuration | Problem on KubeVirt | Conversion Fix |
|-------------------|---------------------|----------------|
| Xen/Nitro drivers | Incompatible with KVM | Install VirtIO drivers |
| cloud-init for EC2 | Looks for metadata service | Remove or reconfigure |
| AWS agents (SSM, etc.) | Unnecessary, may cause issues | Remove agents |
| Bootloader config | May reference AWS devices | Update for virtio devices |

## Conversion Method

EC2 uses **disk-based in-place conversion** with virt-v2v:

```
                    Conversion Pod
                    ┌─────────────────────────────────────────┐
                    │                                         │
   PVC (disk-0) ────┼──▶ /dev/block0 ──┐                      │
                    │                  │                      │
   PVC (disk-1) ────┼──▶ /dev/block1 ──┼──▶ virt-v2v-in-place │
                    │                  │      (modifies       │
   PVC (disk-N) ────┼──▶ /dev/blockN ──┘       disks)         │
                    │                                         │
                    └─────────────────────────────────────────┘
```

The conversion happens **in-place** on the block devices. No data is copied - virt-v2v directly modifies the disk contents.

## What virt-v2v Does

1. **Detects the guest OS** - Identifies Linux distribution or Windows version
2. **Installs VirtIO drivers** - Adds virtio-blk, virtio-net, virtio-scsi drivers
3. **Updates bootloader** - Configures GRUB/bootloader for new device names
4. **Removes cloud agents** - Disables EC2-specific services
5. **Adjusts network config** - Prepares for VirtIO network interfaces

## EC2 vs vSphere Conversion

| Aspect | EC2 | vSphere |
|--------|-----|---------|
| Input source | `-i disk` (block devices) | `-i libvirtxml` (VM definition) |
| VM metadata | Not needed | Fetched from vCenter |
| Disk access | Direct block devices | Same |
| Conversion tool | virt-v2v-in-place | virt-v2v-in-place |

EC2 is simpler because there's no hypervisor metadata to fetch - just the raw disks.

## Skipping Conversion

You can skip guest conversion by setting `skipGuestConversion: true` in the Plan spec.

