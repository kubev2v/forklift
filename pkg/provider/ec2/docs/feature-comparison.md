# Feature Comparison

This document compares the EC2 provider with VMware (vSphere) and oVirt providers.

| Feature | EC2 | vSphere | oVirt |
|---------|-----|---------|-------|
| **Inventory & Discovery** | | | |
| Discovery Method | Polling | Event-driven | Polling |
| Tag/Label Filtering | Yes | No | No |
| Watch (WebSockets) | Yes | Yes | Yes |
| Detail Levels | No | Yes | Yes |
| **Compute & Hardware** | | | |
| CPU Topology | Fixed from instance type | Configurable | Configurable |
| Memory | Fixed from instance type | Source-dependent | Source-dependent |
| Firmware (BIOS/UEFI) | Yes | Yes | Yes |
| Secure Boot | No | Yes | Yes |
| TPM | No | Yes | Yes |
| Nested Virtualization | Yes (.metal only) | Yes | Yes |
| **Storage & Networking** | | | |
| Disk Bus | VirtIO, SATA | VirtIO, SATA, SCSI, IDE, NVMe | VirtIO, SCSI, SATA, IDE |
| NIC Model | VirtIO, E1000e | VirtIO, E1000e | Source-dependent |
| MAC Preservation | Yes | Yes | Yes |
| Static IP Preservation | No | Yes | No |
| Shared Disks | No | Yes | Yes |
| LUKS Encryption | No | Yes | Yes |
| **Migration Types** | | | |
| Cold Migration | Yes | Yes | Yes |
| Warm Migration | No | Yes | Yes |
| Live Migration | No | Yes | No |
| **Naming & Templating** | | | |
| PVC Name Template | No | Yes | No |
| Volume Name Template | No | Yes | No |
| Network Name Template | No | Yes | No |
| Custom VM Name | Yes | Yes | Yes |
| **KubeVirt Integration** | | | |
| Template Labels | Auto-detected | Mapped | Mapped |
| VM Preferences | No | Yes | Yes |
| Instance Types | No | Yes | No |
| Compatibility Mode | Yes | Yes | No |
| Migration Hooks | Yes | Yes | Yes |
| **Resource Scope** | | | |
| Compute | Instances | VMs, Hosts, Clusters | VMs, Hosts, Clusters |
| Network | VPCs, Subnets | Networks, Port Groups | Networks, NIC Profiles |
| Storage | EBS Types, Volumes | Datastores | Storage Domains |

EC2 uniquely supports filtering VMs by AWS tags via `?label.key=value` query parameters.

EC2 only supports cold migration due to its snapshot-based transfer model.

## Limitations Summary

| Limitation | Reason |
|------------|--------|
| Cold migration only | Snapshot-based transfer |
| No static IP preservation | Different network model |
| Same region only (cross-account) | Snapshot sharing limitation |
| EBS volumes only | Instance store not supported |
