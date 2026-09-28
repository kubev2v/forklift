# Azure Resource Tagging

The Azure provider uses Azure tags on snapshots and Kubernetes labels on VolumeSnapshot/PVC objects to track migration resources. This dual approach makes migrations resilient to controller restarts and enables cleanup of orphaned resources on both sides.

## Why Tags?

| Benefit | Description |
|---------|-------------|
| **Resilience** | Controller restarts don't lose track of Azure or Kubernetes resources |
| **Idempotency** | Snapshot creation and VolumeSnapshot provisioning can be safely retried |
| **Visibility** | Resources are discoverable in the Azure Portal and via `kubectl` |
| **Cleanup** | Failed migrations can be cleaned up by tag/label |

## Azure Snapshot Tags

When the provider creates managed disk snapshots:

> **Note:** Azure resource tags use `-` as the separator (e.g., `forklift.konveyor.io-vmID`) because Azure forbids `/` in tag names. Kubernetes labels retain the standard `/` separator.

| Tag Key | Example Value | Purpose |
|---------|---------------|---------|
| `forklift.konveyor.io-vmID` | `/subscriptions/.../virtualMachines/my-vm` | Links to source VM |
| `forklift.konveyor.io-vm-name` | `my-web-server` | Human-readable VM name |
| `forklift.konveyor.io-disk` | `/subscriptions/.../disks/my-vm-osdisk` | Source managed disk resource ID |
| `forklift.konveyor.io-index` | `0` | Disk index (0 = OS disk, 1+ = data disks) |

Snapshot names follow the pattern: `fklft-<vm-name>-<disk-name>-<index>` (truncated to 80 characters).

## Kubernetes Resource Labels

VolumeSnapshotContent, VolumeSnapshot, and PVC objects are labeled for tracking:

| Label Key | Example Value | Purpose |
|-----------|---------------|---------|
| `forklift.konveyor.io/vmID` | `vm-resource-id` | Links all resources to a source VM |
| `forklift.konveyor.io/disk-index` | `0` | Maps to the disk index |

PVCs also carry an annotation linking back to their VolumeSnapshot:

| Annotation Key | Example Value | Purpose |
|----------------|---------------|---------|
| `forklift.konveyor.io/volumeSnapshot` | `my-vm-snap-0` | VolumeSnapshot name for OwnerRef injection |

## How Tags Are Used

**During migration:**
1. Deallocate source VM
2. Create Azure managed disk snapshots → tag with vmID and disk info
3. Query snapshots by naming convention to check provisioning state
4. Create VolumeSnapshotContent/VolumeSnapshot/PVC → label with vmID
5. Inject OwnerReferences (PVC → VolumeSnapshot) for cascading deletion

**For recovery:**
- If the controller restarts mid-migration, it queries Azure snapshots by name convention and Kubernetes resources by label
- Existing resources are discovered and migration continues from where it left off

## Finding Tagged Resources

### Azure Portal / CLI

```bash
# Find snapshots for a specific VM
az snapshot list --resource-group my-rg \
  --query "[?tags.\"forklift.konveyor.io-vmID\"=='vm-id']"

# Find all Forklift snapshots
az snapshot list --resource-group my-rg \
  --query "[?starts_with(name, 'fklft-')]"
```

### Kubernetes

```bash
# Find VolumeSnapshots for a specific VM
kubectl get volumesnapshots -n openshift-mtv \
  -l forklift.konveyor.io/vmID=<vm-id>

# Find PVCs for a specific VM
kubectl get pvc -n openshift-mtv \
  -l forklift.konveyor.io/vmID=<vm-id>

# Find VolumeSnapshotContents (cluster-scoped)
kubectl get volumesnapshotcontents \
  -l forklift.konveyor.io/vmID=<vm-id>
```

## Cleanup

After successful migration:
- **VolumeSnapshots** are retained but owned by PVCs via OwnerReferences. Deleting the PVC cascades to delete the VolumeSnapshot and its underlying Azure snapshot.
- **Azure snapshots** are deleted by the CSI driver when VolumeSnapshotContent objects are deleted (deletionPolicy: Delete).
- **PVCs** are managed through the normal KubeVirt VM lifecycle.

For failed migrations, the ensurer provides `DeleteVolumeSnapshots` and `DeleteVolumeSnapshotContents` methods to clean up Kubernetes resources, which in turn trigger Azure snapshot cleanup via the CSI driver.
