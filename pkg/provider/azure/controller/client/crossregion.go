package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"k8s.io/utils/ptr"
)

// CopySnapshotsCrossRegion copies each source-region snapshot to the target region.
// Uses Azure's CopyStart create option which performs a server-side async copy.
func (r *Client) CopySnapshotsCrossRegion(vmRef ref.Ref) ([]string, error) {
	if !r.IsCrossRegion() {
		return nil, fmt.Errorf("cross-region copy requested but targetRegion is not set")
	}

	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return nil, err
	}

	sourceNames, err := r.GetSnapshotsForVM(vmRef)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(sourceNames) == 0 {
		return nil, fmt.Errorf("no source snapshots found for VM %s", vmRef.Name)
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()
	targetRegion := r.GetTargetRegion()

	var crossRegionNames []string
	for i, srcName := range sourceNames {
		srcResourceID, err := r.GetSnapshotResourceID(srcName)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		xrName := fmt.Sprintf("fklft-xr-%s-%d", vmRef.Name, i)
		if len(xrName) > 80 {
			xrName = xrName[:80]
		}

		sku := r.getSnapshotSku()
		snapshot := armcompute.Snapshot{
			Location: ptr.To(targetRegion),
			Properties: &armcompute.SnapshotProperties{
				CreationData: &armcompute.CreationData{
					CreateOption:     ptr.To(armcompute.DiskCreateOptionCopyStart),
					SourceResourceID: ptr.To(srcResourceID),
				},
			},
			SKU: &armcompute.SnapshotSKU{
				Name: ptr.To(armcompute.SnapshotStorageAccountTypes(sku)),
			},
			Tags: map[string]*string{
				azure.TagVMID:        ptr.To(vmRef.ID),
				azure.TagVMName:      ptr.To(vmRef.Name),
				azure.TagCrossRegion: ptr.To("true"),
				azure.TagSource:      ptr.To(srcName),
				azure.TagIndex:       ptr.To(fmt.Sprintf("%d", i)),
			},
		}

		poller, err := snapshotClient.BeginCreateOrUpdate(ctx, snapshotRG, xrName, snapshot, nil)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		crossRegionNames = append(crossRegionNames, xrName)
		log.Info("Cross-region snapshot copy initiated",
			"vm", vmRef.Name,
			"source", srcName,
			"target", xrName,
			"targetRegion", targetRegion)
	}

	return crossRegionNames, nil
}

// AreCrossRegionSnapshotsReady checks whether all cross-region snapshot copies have completed.
func (r *Client) AreCrossRegionSnapshotsReady(vmRef ref.Ref, crossRegionNames []string) (bool, error) {
	snapshotClient, err := r.getSnapshotClient()
	if err != nil {
		return false, err
	}

	ctx := context.Background()
	snapshotRG := r.getSnapshotResourceGroup()

	for _, name := range crossRegionNames {
		result, err := snapshotClient.Get(ctx, snapshotRG, name, nil)
		if err != nil {
			return false, liberr.Wrap(err)
		}

		if result.Properties == nil || result.Properties.ProvisioningState == nil {
			return false, nil
		}

		state := *result.Properties.ProvisioningState
		if state != "Succeeded" {
			log.V(1).Info("Cross-region snapshot not ready", "snapshot", name, "state", state)
			return false, nil
		}
	}

	return true, nil
}

// GetCrossRegionSnapshotNames returns the expected cross-region snapshot names for a VM.
func (r *Client) GetCrossRegionSnapshotNames(vmRef ref.Ref) ([]string, error) {
	sourceNames, err := r.GetSnapshotsForVM(vmRef)
	if err != nil {
		return nil, err
	}

	var xrNames []string
	for i := range sourceNames {
		xrName := fmt.Sprintf("fklft-xr-%s-%d", vmRef.Name, i)
		if len(xrName) > 80 {
			xrName = xrName[:80]
		}
		xrNames = append(xrNames, xrName)
	}
	return xrNames, nil
}

// GetCrossRegionSnapshotResourceID returns the Azure resource ID for a cross-region snapshot.
func (r *Client) GetCrossRegionSnapshotResourceID(snapshotName string) (string, error) {
	rg := r.getSnapshotResourceGroup()
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s",
		r.subscriptionID, rg, snapshotName), nil
}
