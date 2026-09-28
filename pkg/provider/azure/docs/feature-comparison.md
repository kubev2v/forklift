# Feature Comparison

This document compares the Azure provider with EC2, VMware (vSphere), and oVirt providers.

| Feature | Azure | EC2 | vSphere | oVirt |
|---------|-------|-----|---------|-------|
| **Inventory & Discovery** | | | | |
| Discovery Method | Polling | Polling | Event-driven | Polling |
| Tag/Label Filtering | No | Yes | No | No |
| Watch (WebSockets) | Yes | Yes | Yes | Yes |
| Detail Levels | No | No | Yes | Yes |
| **Compute & Hardware** | | | | |
| CPU Topology | Mapped from VM size | Fixed from instance type | Configurable | Configurable |
| Memory | Mapped from VM size | Fixed from instance type | Source-dependent | Source-dependent |
| Firmware (BIOS/UEFI) | Yes (Gen1/Gen2) | Yes | Yes | Yes |
| Secure Boot | No | No | Yes | Yes |
| TPM | No | No | Yes | Yes |
| **Storage & Networking** | | | | |
| Disk Provisioning | CSI snapshot restore | Direct EBS volume | DataVolume/CDI | DataVolume/CDI |
| Disk Bus | VirtIO | VirtIO, SATA | VirtIO, SATA, SCSI, IDE, NVMe | VirtIO, SCSI, SATA, IDE |
| NIC Model | VirtIO | VirtIO, E1000e | VirtIO, E1000e | Source-dependent |
| MAC Preservation | No | Yes | Yes | Yes |
| Static IP Preservation | No | No | Yes | No |
| Shared Disks | No | No | Yes | Yes |
| **Migration Types** | | | | |
| Cold Migration | **Yes** | Yes | Yes | Yes |
| Warm Migration | No | No | Yes | Yes |
| Live Migration | No | No | Yes | No |
| **Guest Conversion** | | | | |
| virt-v2v Required | **Optional** (skippable) | Yes | Yes | Yes |
| Compatibility Mode | SATA + E1000e | VirtIO drivers | VirtIO drivers | VirtIO drivers |
| **Naming & Templating** | | | | |
| PVC Name Template | No | No | Yes | No |
| Custom VM Name | Yes | Yes | Yes | Yes |
| **KubeVirt Integration** | | | | |
| Template Labels | Auto-detected | Auto-detected | Mapped | Mapped |
| VM Preferences | No | No | Yes | Yes |
| Instance Types | No | No | Yes | No |
| Migration Hooks | Yes | Yes | Yes | Yes |
| **Resource Scope** | | | | |
| Compute | VMs | Instances | VMs, Hosts, Clusters | VMs, Hosts, Clusters |
| Network | Subnets (within VNets) | VPCs, Subnets | Networks, Port Groups | Networks, NIC Profiles |
| Storage | Disk SKUs (types) | EBS Types, Volumes | Datastores | Storage Domains |

## Azure-Specific Advantages

| Advantage | Detail |
|-----------|--------|
| Skippable guest conversion | Azure VMs use Hyper-V paravirtual drivers natively; virt-v2v can be skipped |
| Faster migrations | Skipping conversion eliminates the conversion phase entirely |
| Broader OS support | Any OS that runs on Azure can migrate without driver concerns when skipping conversion |
| CSI-native provisioning | Uses standard Kubernetes VolumeSnapshot API instead of direct cloud API for disks |
| Cross-AZ support | `Standard_ZRS` snapshots work across availability zones (set `snapshotSku: Standard_ZRS` in provider settings) |
| Cross-region support | Server-side snapshot copy via `CopyStart` (set `targetRegion` in provider settings) |

## Limitations Summary

| Limitation | Reason |
|------------|--------|
| Cold migration only | Requires VM deallocation for consistent snapshots |
| No MAC preservation | Azure NIC MAC addresses are not exposed in the VM model |
| Managed disks only | Classic/unmanaged disks are not supported |
| No static IP preservation | Different network model between Azure VNet and OVN |
| No warm/live migration | Not supported |
