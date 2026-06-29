# Azure Provider

Migrate Azure Virtual Machines to OpenShift Virtualization using managed disk snapshots and Kubernetes CSI VolumeSnapshot integration.

## Overview

Cold migration of Azure VMs via managed disk snapshot integration. Leverages the Azure Disk CSI driver (`disk.csi.azure.com`) and Kubernetes VolumeSnapshot/VolumeSnapshotContent API for zero-copy disk provisioning.

By default, guest OS conversion via virt-v2v runs as part of the migration (same as EC2, vSphere, and other providers). Users who want to skip conversion can set `plan.spec.skipGuestConversion: true` -- Azure VMs already use Hyper-V compatible drivers, so skipping is safe in most cases. When skipping conversion, setting `plan.spec.useCompatibilityMode: true` selects SATA bus for guaranteed bootability; otherwise VirtIO devices are used.

**Pipeline:** Pre-Snapshot (VM running) -> Deallocate -> Final Snapshot (incremental, fast) -> [Cross-Region Copy] -> VolumeSnapshotContent -> VolumeSnapshot -> PVC (CSI restore) -> [Guest Conversion] -> KubeVirt VM

### Migration Flow

1. **Pre-Snapshot** (VM running): Incremental crash-consistent snapshots are created for each managed disk while the VM is still running. This pre-warms Azure's incremental snapshot tracking.
2. **VM Deallocation**: The source Azure VM is deallocated to ensure consistent disk state.
3. **Final Snapshot** (incremental): A second set of incremental snapshots is created. Because Azure already has the pre-snapshot data, only the blocks that changed since step 1 need to be captured -- making this step significantly faster.
4. **Delete Pre-Snapshots**: The pre-snapshots from step 1 are deleted (no longer needed). Azure incremental snapshots are independently restorable.
5. **Cross-Region Copy** (optional): If `targetRegion` is set, the final snapshots are copied to the target region using Azure's server-side `CopyStart` operation.
6. **VolumeSnapshotContent**: Kubernetes VolumeSnapshotContent objects are created, referencing the final Azure snapshot resource IDs (or cross-region copies) via the CSI `snapshotHandle`.
7. **VolumeSnapshot**: Kubernetes VolumeSnapshot objects are created, bound to the VolumeSnapshotContent objects.
8. **PVC Provisioning**: PersistentVolumeClaims are created with `dataSource` pointing to the VolumeSnapshots. The Azure Disk CSI driver provisions new managed disks from the snapshots.
9. **OwnerReference Injection**: OwnerReferences are set on VolumeSnapshots so that deleting a PVC cascades to clean up the snapshot chain.
10. **Guest Conversion** (default): virt-v2v runs to convert the guest OS for optimal KubeVirt compatibility. Skipped when `skipGuestConversion: true`.
11. **VM Creation**: KubeVirt VM is created with the migrated PVCs attached.

**Downtime optimization**: The VM is only deallocated at step 2. The pre-snapshot in step 1 captures the bulk of the disk data while the VM remains available. The final snapshot in step 3 only needs to capture the delta, reducing the time the VM spends in a deallocated state before migration completes.

**Storage Note:** Disks are provisioned through the standard Kubernetes CSI snapshot restore flow. The Azure Disk CSI driver handles the actual disk creation from snapshots. No direct Azure API calls are needed for disk provisioning.

## Credentials and RBAC

### Secret Format

The provider secret contains **only the source Azure environment credentials**. The target OCP cluster uses its own identity (Workload Identity / Managed Identity) for the Azure Disk CSI driver.

| Key | Required | Description |
|-----|----------|-------------|
| `tenantId` | **Yes** | Azure AD tenant ID |
| `subscriptionId` | **Yes** | Azure subscription containing source VMs |
| `clientId` | **Yes** | Service principal application (client) ID |
| `clientSecret` | **Yes** | Service principal client secret |
| `resourceGroup` | **Yes** | Azure resource group containing the source VMs |

### Required Azure RBAC Roles

The service principal needs these roles scoped to the **source resource group** (or subscription):

| Role | Purpose |
|------|---------|
| **Reader** | List and inspect VMs, disks, networks, subnets (inventory collection) |
| **Virtual Machine Contributor** | Deallocate and power off source VMs |
| **Disk Snapshot Contributor** | Create and delete managed disk snapshots |

If using **cross-region migration**, the principal also needs **Disk Snapshot Contributor** on the target-region resource group (or the snapshot resource group if `snapshotResourceGroup` is set).

No roles are needed on the target OCP cluster side -- the CSI driver uses the cluster's own identity.

## Setup with kubectl-mtv

### 1. Create the Azure Provider

```bash
kubectl mtv create provider my-azure --type azure \
  --azure-tenant-id "$AZURE_TENANT_ID" \
  --azure-subscription-id "$AZURE_SUBSCRIPTION_ID" \
  --azure-client-id "$AZURE_CLIENT_ID" \
  --azure-client-secret "$AZURE_CLIENT_SECRET" \
  --azure-resource-group "my-resource-group"
```

### Provider Flags Reference

| Flag | Required | Description |
|------|----------|-------------|
| `--type azure` | **Yes** | Provider type |
| `--azure-tenant-id` | **Yes** | Azure Active Directory tenant ID |
| `--azure-subscription-id` | **Yes** | Azure subscription ID containing the source VMs |
| `--azure-client-id` | **Yes** | Service principal application (client) ID |
| `--azure-client-secret` | **Yes** | Service principal client secret |
| `--azure-resource-group` | **Yes** | Resource group containing the source VMs |

### 2. Explore the Inventory

After provider creation, explore available resources:

```bash
# List Azure VMs
kubectl mtv get inventory azure-vm my-azure

# List managed disks
kubectl mtv get inventory azure-disk my-azure

# List networks (subnets)
kubectl mtv get inventory azure-network my-azure

# List disk types (for storage mapping)
kubectl mtv get inventory azure-disk-type my-azure
```

## Manual Setup (YAML)

If you prefer YAML manifests:

### Provider Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  tenantId: "00000000-0000-0000-0000-000000000000"
  subscriptionId: "00000000-0000-0000-0000-000000000000"
  clientId: "00000000-0000-0000-0000-000000000000"
  clientSecret: "your-client-secret"
  resourceGroup: "my-resource-group"
```

### Provider Resource

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-azure
  namespace: openshift-mtv
spec:
  type: azure
  secret:
    name: azure-credentials
    namespace: openshift-mtv
  # Optional settings for migration behavior:
  # settings:
  #   targetRegion: "westus2"
```

#### Provider Settings

All provider settings are optional and control migration behavior:

| Setting | Required | Description |
|---------|----------|-------------|
| `snapshotResourceGroup` | No | Resource group for snapshots (defaults to the secret's `resourceGroup`) |
| `snapshotSku` | No | Snapshot SKU: `Standard_LRS` (default), `Standard_ZRS`, `Premium_LRS` |
| `targetRegion` | No | Target Azure region for cross-region migration |
| `volumeSnapshotClassName` | No | VolumeSnapshotClass name (auto-discovered from `disk.csi.azure.com` driver if not set) |

**Why Standard_LRS?** Locally Redundant Storage is supported in all Azure regions for incremental snapshots. For cross-availability-zone migrations, set `snapshotSku` to `Standard_ZRS` (only available in regions that support ZRS).

### Cross-Resource-Group Migration

The source VMs and the target OCP cluster do not need to be in the same resource group. The VolumeSnapshotContent references snapshots by their fully-qualified Azure resource ID, so the CSI driver can resolve snapshots from any RG.

However, the **CSI driver's identity** (the cluster's Managed Identity) must be able to read the snapshot. Two options:

1. **Grant Reader on the snapshot RG** -- Assign the cluster identity `Reader` on the resource group where Forklift creates snapshots.
2. **Use `snapshotResourceGroup`** (recommended) -- Set this provider setting to a resource group the CSI driver already has access to (e.g., the cluster's node/infrastructure RG). Forklift creates snapshots there instead of the source VM's RG, avoiding extra cross-RG role assignments.

## Cross-Availability-Zone Migration

Azure snapshots support cross-AZ migration within the same region:

| Snapshot SKU | Cross-AZ Support | Use Case |
|-------------|-------------------|----------|
| `Standard_LRS` (default) | Same AZ only | Universally supported; lower cost |
| `Standard_ZRS` | **Yes** | VMs and target nodes may be in different AZs (region must support ZRS) |
| `Premium_LRS` | Same AZ only | Higher performance, same-AZ only |

For cross-AZ migration, ensure the target StorageClass uses `WaitForFirstConsumer` volume binding mode so the CSI driver provisions the disk in the correct AZ based on pod scheduling.

## Cross-Region Migration

When the source VMs and the target OpenShift cluster are in **different Azure regions**, set the `targetRegion` provider setting:

```yaml
spec:
  settings:
    targetRegion: "westus2"  # Region where the OCP cluster runs
```

This adds two phases to the migration:

1. **CopySnapshotsCrossRegion**: Initiates server-side async snapshot copies from the source region to the target region using Azure's `CopyStart` operation.
2. **WaitForCrossRegionSnapshots**: Polls until all cross-region copies complete.

The subsequent VolumeSnapshotContent objects reference the **target-region** snapshot IDs, ensuring the Azure Disk CSI driver can provision disks locally.

Both source-region and cross-region snapshots are cleaned up automatically on migration completion or failure.

## Guest Conversion

By default, virt-v2v guest conversion runs during migration (same as EC2, vSphere, and other providers). This ensures optimal driver and configuration compatibility with KubeVirt.

Azure VMs run on the Hyper-V hypervisor. Windows guests include Hyper-V Integration Services, and Linux guests include Linux Integration Services (LIS) providing synthetic drivers compatible with KubeVirt, so skipping conversion is safe in most cases -- particularly for Windows VMs:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Plan
spec:
  skipGuestConversion: true
  # useCompatibilityMode: true  # optional; uses SATA bus when skipping conversion
```

When conversion is skipped, the `useCompatibilityMode` flag controls device selection (both flags must be set explicitly):
- `skipGuestConversion: true` + `useCompatibilityMode: true`: Use compatibility devices (SATA bus, E1000E NIC, USB input) for guaranteed bootability without VirtIO drivers
- `skipGuestConversion: true` + `useCompatibilityMode: false` (or unset): Use high-performance VirtIO devices (requires VirtIO drivers already installed in the guest OS)

Gen2 Azure VMs are configured with UEFI firmware and SMM (System Management Mode) for Secure Boot compatibility. Gen1 VMs use BIOS firmware.

## Requirements

**Source Environment:**
- Azure VMs with managed disks only (unmanaged/classic disks are not supported)
- Service principal with Reader, Virtual Machine Contributor, and Disk Snapshot Contributor roles (see RBAC section above)

**Target Environment:**
- OpenShift cluster running on Azure (ARO, self-managed, or IPI)
- Azure Disk CSI driver (`disk.csi.azure.com`) installed and operational
- VolumeSnapshot CRDs and a VolumeSnapshotClass for `disk.csi.azure.com` installed
- StorageClass backed by the Azure Disk CSI driver

## Architecture

```
pkg/provider/azure/
├── docs/             # Supplementary design docs (CSI integration, feature comparison, tagging)
├── inventory/
│   ├── client/       # Azure SDK wrapper (VMs, Disks, VNets, Subnets)
│   ├── collector/    # Polling-based inventory collector
│   ├── model/        # SQLite DB models (VM, Disk, Network, Storage)
│   └── web/          # REST API handlers, Finder, Resolver
├── controller/
│   ├── adapter/      # Adapter factory (wires Builder, Client, Validator, Ensurer)
│   ├── builder/      # VM spec builder (compatibility mode, firmware), VolumeSnapshot/PVC builders
│   ├── client/       # Azure migration client (deallocate, snapshot, cross-region copy, power state)
│   ├── ensurer/      # Kubernetes resource ensurer (VolumeSnapshotContent, VolumeSnapshot, PVC)
│   ├── handler/      # Plan, NetworkMap, StorageMap event handlers
│   ├── inventory/    # Controller-side inventory helpers
│   ├── mapping/      # Storage/network mapping lookup utilities
│   ├── migrator/     # Migration itinerary, phase executor, pipeline
│   ├── scheduler/    # MaxInFlight concurrency limiter
│   └── validator/    # Pre-migration validation (disks, mappings, cold-only)
└── testutil/         # Mock Azure API and test fixtures
```
