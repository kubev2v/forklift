package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"k8s.io/utils/ptr"
)

// CreateDiskSnapshots creates incremental managed disk snapshots for all disks of a VM.
func (r *Client) CreateDiskSnapshots(vmRef ref.Ref) ([]string, error) {
	return r.createDiskSnapshotsWithPrefix(vmRef, "fklft")
}

// CreatePreSnapshots creates incremental pre-snapshots (while VM is running) to pre-warm
// Azure's incremental tracking. The final snapshot after deallocation will be faster.
func (r *Client) CreatePreSnapshots(vmRef ref.Ref) ([]string, error) {
	return r.createDiskSnapshotsWithPrefix(vmRef, "fklft-pre")
}

func (r *Client) createDiskSnapshotsWithPrefix(vmRef ref.Ref, prefix string) ([]string, error) {
	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return nil, err
	}

	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vmRef)
	if err != nil {
		log.Error(err, "Failed to get Azure VM from inventory", "vmRef", vmRef)
		return nil, liberr.Wrap(err)
	}

	log.Info("Azure VM details for snapshot creation",
		"vm", vmRef.Name,
		"managedDisksCount", len(azureVM.ManagedDisks),
		"disksCount", len(azureVM.Disks),
		"hasProperties", azureVM.Properties != nil)

	diskIDs := inventory.GetManagedDiskIDs(azureVM)
	if len(diskIDs) == 0 {
		log.Info("No managed disk IDs found, dumping VM details",
			"vm", vmRef.Name,
			"managedDisks", azureVM.ManagedDisks,
			"disks", azureVM.Disks)
		return nil, fmt.Errorf("no managed disks found for VM %s", vmRef.Name)
	}

	log.Info("Found managed disk IDs for snapshot",
		"vm", vmRef.Name,
		"diskIDs", diskIDs)

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()
	sku := r.getSnapshotSku()

	var snapshotNames []string
	for i, diskID := range diskIDs {
		diskName := extractDiskName(diskID)
		snapshotName := fmt.Sprintf("%s-%s-%s-%d", prefix, vmRef.Name, diskName, i)
		if len(snapshotName) > 80 {
			snapshotName = snapshotName[:80]
		}

		snapshot := armcompute.Snapshot{
			Location: azureVM.Location,
			Properties: &armcompute.SnapshotProperties{
				Incremental: ptr.To(true),
				CreationData: &armcompute.CreationData{
					CreateOption:     ptr.To(armcompute.DiskCreateOptionCopy),
					SourceResourceID: ptr.To(diskID),
				},
			},
			SKU: &armcompute.SnapshotSKU{
				Name: ptr.To(armcompute.SnapshotStorageAccountTypes(sku)),
			},
			Tags: map[string]*string{
				azure.TagVMID:   ptr.To(vmRef.ID),
				azure.TagVMName: ptr.To(vmRef.Name),
				azure.TagDisk:   ptr.To(diskID),
				azure.TagIndex:  ptr.To(fmt.Sprintf("%d", i)),
			},
		}

		poller, err := snapshotClient.BeginCreateOrUpdate(ctx, snapshotRG, snapshotName, snapshot, nil)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		snapshotNames = append(snapshotNames, snapshotName)
		log.Info("Snapshot created", "vm", vmRef.Name, "disk", diskName, "snapshot", snapshotName, "prefix", prefix)
	}

	return snapshotNames, nil
}

func (r *Client) GetSnapshotsForVM(vmRef ref.Ref) ([]string, error) {
	return r.getSnapshotsForVMWithPrefix(vmRef, "fklft")
}

func (r *Client) getSnapshotsForVMWithPrefix(vmRef ref.Ref, prefix string) ([]string, error) {
	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return nil, err
	}

	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	diskIDs := inventory.GetManagedDiskIDs(azureVM)
	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()

	var snapshotNames []string
	for i, diskID := range diskIDs {
		diskName := extractDiskName(diskID)
		snapshotName := fmt.Sprintf("%s-%s-%s-%d", prefix, vmRef.Name, diskName, i)
		if len(snapshotName) > 80 {
			snapshotName = snapshotName[:80]
		}

		_, err := snapshotClient.Get(ctx, snapshotRG, snapshotName, nil)
		if err == nil {
			snapshotNames = append(snapshotNames, snapshotName)
		}
	}

	return snapshotNames, nil
}

// GetPreSnapshotsForVM returns pre-snapshot names that exist for a VM.
func (r *Client) GetPreSnapshotsForVM(vmRef ref.Ref) ([]string, error) {
	return r.getSnapshotsForVMWithPrefix(vmRef, "fklft-pre")
}

// DeletePreSnapshots deletes all pre-snapshots for a VM.
func (r *Client) DeletePreSnapshots(vmRef ref.Ref) error {
	preSnaps, err := r.GetPreSnapshotsForVM(vmRef)
	if err != nil {
		return liberr.Wrap(err)
	}

	if len(preSnaps) == 0 {
		return nil
	}

	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()

	for _, name := range preSnaps {
		if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, name); err != nil {
			log.Error(err, "Failed to delete pre-snapshot", "snapshot", name)
		} else {
			log.Info("Deleted pre-snapshot", "snapshot", name)
		}
	}

	return nil
}

// DeleteSnapshots deletes all final snapshots for a VM.
func (r *Client) DeleteSnapshots(vmRef ref.Ref) error {
	snaps, err := r.GetSnapshotsForVM(vmRef)
	if err != nil {
		return liberr.Wrap(err)
	}

	if len(snaps) == 0 {
		return nil
	}

	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()

	for _, name := range snaps {
		if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, name); err != nil {
			log.Error(err, "Failed to delete snapshot", "snapshot", name)
		} else {
			log.Info("Deleted snapshot", "snapshot", name)
		}
	}

	if r.IsCrossRegion() {
		xrNames, xrErr := r.GetCrossRegionSnapshotNames(vmRef)
		if xrErr != nil {
			log.Error(xrErr, "Failed to get cross-region snapshot names for cleanup", "vm", vmRef.Name)
		} else {
			for _, xrName := range xrNames {
				if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, xrName); err != nil {
					log.Error(err, "Failed to delete cross-region snapshot", "snapshot", xrName)
				} else {
					log.Info("Deleted cross-region snapshot", "snapshot", xrName)
				}
			}
		}
	}

	return nil
}

func (r *Client) IsSnapshotReady(snapshotName string) (bool, error) {
	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return false, err
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()

	result, err := snapshotClient.Get(ctx, snapshotRG, snapshotName, nil)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if result.Properties == nil || result.Properties.ProvisioningState == nil {
		return false, nil
	}

	return *result.Properties.ProvisioningState == "Succeeded", nil
}

func (r *Client) GetSnapshotResourceID(snapshotName string) (string, error) {
	rg := r.getSnapshotResourceGroup()
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s",
		r.subscriptionID, rg, snapshotName), nil
}

func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (string, string, error) {
	names, err := r.CreateDiskSnapshots(vmRef)
	if err != nil {
		return "", "", err
	}
	return strings.Join(names, ","), "", nil
}

func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (string, error) {
	if snapshot == "" {
		return "", nil
	}

	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()
	snapshotNames := strings.Split(snapshot, ",")

	for _, snapshotName := range snapshotNames {
		if snapshotName == "" {
			continue
		}
		if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, snapshotName); err != nil {
			return "", err
		}
	}

	// Safety-net cleanup of pre-snapshots (normally deleted early in the pipeline)
	preSnaps, preErr := r.GetPreSnapshotsForVM(vmRef)
	if preErr != nil {
		log.Error(preErr, "Failed to get pre-snapshot names for cleanup", "vm", vmRef.Name)
	} else {
		for _, preName := range preSnaps {
			if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, preName); err != nil {
				log.Error(err, "Failed to delete pre-snapshot", "snapshot", preName)
			}
		}
	}

	// Also clean up cross-region copies if they exist
	if r.IsCrossRegion() {
		xrNames, err := r.GetCrossRegionSnapshotNames(vmRef)
		if err != nil {
			log.Error(err, "Failed to get cross-region snapshot names for cleanup", "vm", vmRef.Name)
		} else {
			for _, xrName := range xrNames {
				if err := r.deleteSnapshot(ctx, snapshotClient, snapshotRG, xrName); err != nil {
					log.Error(err, "Failed to delete cross-region snapshot", "snapshot", xrName)
				}
			}
		}
	}

	return "", nil
}

func (r *Client) deleteSnapshot(ctx context.Context, client SnapshotAPI, resourceGroup, snapshotName string) error {
	poller, err := client.BeginDelete(ctx, resourceGroup, snapshotName, nil)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		return liberr.Wrap(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, string, error) {
	snapshotNames := strings.Split(precopy.Snapshot, ",")
	for _, name := range snapshotNames {
		if name == "" {
			continue
		}
		ready, err := r.IsSnapshotReady(name)
		if err != nil {
			return false, "", err
		}
		if !ready {
			return false, precopy.Snapshot, nil
		}
	}
	return true, precopy.Snapshot, nil
}

func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	return true, nil
}
