# Migration Features Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details migration types, guest conversion, and storage features supported by each provider.

## Migration Types

Forklift supports four migration types, specified via `spec.type` in the Plan CR:

| Type | Description |
|------|-------------|
| `cold` | VM is shut down before migration. Most reliable method. |
| `warm` | Initial disk copy while VM runs, brief downtime for final sync. |
| `live` | Minimal downtime migration using KubeVirt live migration. |
| `conversion` | Only perform guest OS conversion without disk transfer. |

### Support Matrix

| Migration Type | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|----------------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `cold` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `warm` | Yes | Yes* | No | No | No | No | No |
| `live` | No | No | No | Yes** | No | No | No |
| `conversion` | Yes | No | No | No | No | No | No |

*oVirt warm migration requires feature gate `FEATURE_OVIRT_WARM_MIGRATION`
**OpenShift live migration requires feature gate `FEATURE_OCP_LIVE_MIGRATION`

### Warm Migration

Warm migration uses incremental snapshots to minimize cutover downtime:

1. Initial full disk copy while source VM continues running
2. Periodic delta syncs capture changes
3. Final cutover with brief downtime

**vSphere Requirements:**
- Changed Block Tracking (CBT) enabled on VMs
- vSphere 6.5+ for incremental backup support
- Feature gate: `FEATURE_VSPHERE_INCREMENTAL_BACKUP` (enabled by default)
- VDDK image must be configured

**oVirt Requirements:**
- Feature gate: `FEATURE_OVIRT_WARM_MIGRATION` (enabled by default)

### Live Migration

Live migration transfers running VMs between OpenShift clusters with minimal downtime using KubeVirt's decentralized live migration feature.

**Important:** Live migration is only supported for **OpenShift-to-OpenShift** migrations. It is not supported for vSphere, oVirt, OpenStack, OVA, EC2, or HyperV sources.

**OpenShift Live Migration Requirements:**
- Feature gate: `FEATURE_OCP_LIVE_MIGRATION` (disabled by default)
- KubeVirt feature gate `DecentralizedLiveMigration` must be enabled on **both** source and destination clusters
- Source VM must have `LiveMigratable` condition
- Storage must support live migration (`StorageLiveMigratable` condition)
- Compatible KubeVirt versions on both clusters

---

## Guest Conversion

Guest conversion (virt-v2v) prepares VMs for KubeVirt by:
- Installing VirtIO drivers
- Configuring boot loader for KVM
- Removing hypervisor-specific tools
- Adjusting device configurations

### Conversion Requirements

| Provider | Requires Conversion | Reason |
|----------|:-------------------:|--------|
| vSphere | Yes | VMware tools removal, VirtIO driver injection |
| oVirt | No | Already uses VirtIO drivers |
| OpenStack | No | Already uses VirtIO drivers |
| OpenShift | No | Already a KubeVirt VM |
| OVA | Yes | VMware OVF format, needs driver injection |
| EC2 | Yes | AWS-specific drivers, needs VirtIO |
| HyperV | Yes | Hyper-V tools removal, VirtIO injection |

### Conversion Options

| Field | Description | Providers |
|-------|-------------|-----------|
| `skipGuestConversion` | Skip virt-v2v entirely (raw copy mode) | vSphere, EC2 |
| `useCompatibilityMode` | Use SATA/E1000E instead of VirtIO when skipping conversion | vSphere, EC2 |
| `installLegacyDrivers` | Install legacy Windows drivers for older OS versions | vSphere, OVA, EC2, HyperV |
| `deleteGuestConversionPod` | Delete conversion pod after successful migration | All with conversion |

**Note:** OVA and HyperV always require virt-v2v as it is used for reading their source formats (OVA files, VHDX).

### Legacy Windows Support

Some older Windows versions require legacy (SHA-1 signed) drivers:
- Windows XP (all versions)
- Windows Server 2003
- Windows Vista (all versions)
- Windows Server 2008
- Windows 7 (pre-SP1)
- Windows Server 2008 R2

Set `installLegacyDrivers: true` for these systems, or leave unset for auto-detection.

---

## Storage Features

### Storage Mapping

All providers support storage class mapping from source to target:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
spec:
  provider:
    source:
      name: vsphere-provider
    destination:
      name: host
  map:
    - source:
        id: datastore-123
      destination:
        storageClass: ocs-storagecluster-ceph-rbd
        volumeMode: Block
        accessMode: ReadWriteOnce
```

### Volume Modes

| Mode | Description | Recommended For |
|------|-------------|-----------------|
| `Filesystem` | PVC mounted as filesystem | General workloads |
| `Block` | Raw block device | High-performance, databases |

### Access Modes

| Mode | Description |
|------|-------------|
| `ReadWriteOnce` | Single node read-write |
| `ReadWriteMany` | Multi-node read-write (requires compatible storage) |
| `ReadOnlyMany` | Multi-node read-only |

### Storage Feature Matrix

| Feature | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|---------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| Volume mode selection | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Access mode selection | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Shared disk migration | Yes | Yes | No | No | No | No | No |
| LUKS decryption | Yes | Yes | No | No | No | No | No |
| Storage offload (XCOPY) | Yes | No | No | No | No | No | No |

### Shared Disks

vSphere and oVirt support migrating shared disks (attached to multiple VMs):

```yaml
spec:
  migrateSharedDisks: true  # default
```

Set to `false` to skip shared disks and avoid duplicate transfers.

### LUKS Encryption

For VMs with LUKS-encrypted disks, provide decryption keys via a Secret:

```yaml
spec:
  vms:
    - id: vm-123
      luks:
        name: luks-keys-secret
        namespace: openshift-mtv
```

Or use Clevis for network-based auto-unlock:

```yaml
spec:
  vms:
    - id: vm-123
      nbdeClevis: true
```

### Storage Offload (XCOPY)

vSphere supports offloading disk copy to storage arrays using XCOPY:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
spec:
  map:
    - source:
        id: datastore-123
      destination:
        storageClass: premium-ssd
      offloadPlugin:
        vsphereXcopyConfig:
          secretRef: storage-credentials
          storageVendorProduct: flashsystem
```

Supported storage vendors:
- `flashsystem` (IBM)
- `vantara` (Hitachi)
- `ontap` (NetApp)
- `primera3par` (HPE)
- `pureFlashArray` (Pure Storage)
- `powerflex`, `powermax`, `powerstore` (Dell)
- `infinibox` (Infinidat)

---

## Network Features

### Network Mapping

All providers support network mapping:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: NetworkMap
spec:
  provider:
    source:
      name: vsphere-provider
    destination:
      name: host
  map:
    - source:
        id: network-123
      destination:
        type: multus
        namespace: default
        name: my-network
```

### Destination Types

| Type | Description |
|------|-------------|
| `pod` | Kubernetes pod network |
| `multus` | Multus CNI additional network |
| `ignored` | Network not mapped (interface skipped) |

### Network Feature Matrix

| Feature | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|---------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| Static IP preservation | Yes | No | No | No | No | No | No |
| MAC address preservation | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| Multiple NICs | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

### Static IP Preservation

vSphere supports preserving static IP configurations:

```yaml
spec:
  preserveStaticIPs: true  # default
```

This injects network configuration scripts during guest conversion to restore static IPs on the target VM.

---

## Transfer Network

Specify a dedicated network for disk transfer traffic:

```yaml
spec:
  transferNetwork:
    name: migration-network
    namespace: openshift-mtv
```

This is useful for:
- Isolating migration traffic
- Using high-bandwidth networks
- Avoiding interference with production traffic
