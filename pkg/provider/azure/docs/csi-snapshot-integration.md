# CSI Snapshot Integration

This document explains how the Azure provider uses Kubernetes VolumeSnapshot APIs and the Azure Disk CSI driver to provision migrated disks without direct Azure volume management.

## Why CSI Snapshots?

The Azure provider differs from the EC2 provider in how disks are provisioned on the target cluster:

| Aspect | EC2 Provider | Azure Provider |
|--------|-------------|----------------|
| Disk provisioning | Direct EBS volume creation via AWS API | CSI snapshot restore via Kubernetes API |
| PV binding | Manual PV/PVC with `volumeHandle` | Dynamic provisioning from VolumeSnapshot `dataSource` |
| Driver dependency | AWS EBS CSI driver (attach only) | Azure Disk CSI driver (snapshot restore + attach) |
| Snapshot lifecycle | Managed via AWS API + tags | Managed via Kubernetes VolumeSnapshot API |

The CSI approach is more Kubernetes-native: the provider only creates the VolumeSnapshotContent to "import" the Azure snapshot, and the CSI driver handles all disk provisioning.

## Resource Chain

```
Azure Managed Disk Snapshot (Azure API)
    │
    ▼
VolumeSnapshotContent (cluster-scoped, references snapshotHandle)
    │
    ▼
VolumeSnapshot (namespaced, bound to VolumeSnapshotContent)
    │
    ▼
PVC (with dataSource: VolumeSnapshot)
    │
    ▼
PV (dynamically provisioned by Azure Disk CSI driver)
    │
    ▼
Azure Managed Disk (new disk created from snapshot)
```

## VolumeSnapshotContent

The VolumeSnapshotContent is the bridge between the Azure snapshot and Kubernetes. It is a cluster-scoped resource that tells the CSI driver where to find the snapshot:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotContent
metadata:
  name: <vm-name>-snapcontent-0
  labels:
    forklift.konveyor.io/vmID: <vm-id>
    forklift.konveyor.io/disk-index: "0"
spec:
  deletionPolicy: Delete
  driver: disk.csi.azure.com
  source:
    snapshotHandle: "/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/snapshots/<name>"
  volumeSnapshotRef:
    kind: VolumeSnapshot
    name: <vm-name>-snap-0
    namespace: <target-namespace>
  volumeSnapshotClassName: csi-azuredisk-vsc
```

Key fields:
- `driver: disk.csi.azure.com` -- tells Kubernetes which CSI driver manages this snapshot
- `source.snapshotHandle` -- the full Azure resource ID of the managed disk snapshot
- `deletionPolicy: Delete` -- when the VolumeSnapshotContent is deleted, the CSI driver deletes the Azure snapshot

## VolumeSnapshot

The VolumeSnapshot is a namespaced resource that binds to the VolumeSnapshotContent:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: <vm-name>-snap-0
  namespace: <target-namespace>
  labels:
    forklift.konveyor.io/vmID: <vm-id>
    forklift.konveyor.io/disk-index: "0"
spec:
  source:
    volumeSnapshotContentName: <vsc-name>
  volumeSnapshotClassName: csi-azuredisk-vsc
```

## PVC with DataSource

The PVC references the VolumeSnapshot as its data source, triggering CSI snapshot restore:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: <vm-name>-disk-0-<random>
  namespace: <target-namespace>
  labels:
    forklift.konveyor.io/vmID: <vm-id>
    forklift.konveyor.io/disk-index: "0"
  annotations:
    forklift.konveyor.io/volumeSnapshot: <vm-name>-snap-0
spec:
  accessModes: [ReadWriteOnce]
  volumeMode: Block
  storageClassName: <from-storage-mapping>
  resources:
    requests:
      storage: <disk-size>
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: <vm-name>-snap-0
```

When this PVC is created, the Azure Disk CSI driver:
1. Reads the VolumeSnapshot → VolumeSnapshotContent → `snapshotHandle`
2. Creates a new Azure managed disk from the snapshot
3. Creates a PV bound to the new disk
4. Binds the PVC to the PV

## OwnerReference Cascade

After PVCs are bound, the provider injects OwnerReferences on VolumeSnapshot objects pointing to their corresponding PVCs:

```
PVC (owner) ──owns──▶ VolumeSnapshot ──refs──▶ VolumeSnapshotContent ──refs──▶ Azure Snapshot
```

Deleting the PVC triggers:
1. VolumeSnapshot is garbage-collected (OwnerReference)
2. VolumeSnapshotContent is cleaned up by the snapshot controller
3. Azure snapshot is deleted by the CSI driver (`deletionPolicy: Delete`)

This ensures no orphaned Azure snapshots accumulate after migration cleanup.

## Prerequisites

The target cluster must have:

1. **Azure Disk CSI driver** -- installed and functioning (`disk.csi.azure.com`)
2. **VolumeSnapshot CRDs** -- the snapshot.storage.k8s.io/v1 API must be available
3. **VolumeSnapshotClass** -- a VolumeSnapshotClass for the Azure Disk CSI driver (auto-discovered by driver name, or set explicitly via provider setting `volumeSnapshotClassName`):

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-azuredisk-vsc
driver: disk.csi.azure.com
deletionPolicy: Delete
```

4. **StorageClass** -- a StorageClass backed by the Azure Disk CSI driver, mapped in the StorageMap
