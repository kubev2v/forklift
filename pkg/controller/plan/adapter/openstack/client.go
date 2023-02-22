package openstack

import (
	"fmt"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/openstack"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Client
type Client struct {
	*plancontext.Context
	OpenstackClient openstack.Client
}

// Connect to the openstack API.
func (r *Client) connect() (err error) {
	r.OpenstackClient.Secret = r.Source.Secret
	r.OpenstackClient.URL = r.Source.Provider.Spec.URL
	err = r.OpenstackClient.Connect()

	return
}

// Power on the source VM.
func (c *Client) PowerOn(vmRef ref.Ref) error {
	return nil
}

// Power off the source VM.
func (c *Client) PowerOff(vmRef ref.Ref) error {
	return nil
}

// Return the source VM's power state.
func (c *Client) PowerState(vmRef ref.Ref) (string, error) {
	// TODO change once we implement warm migration
	return "SHUTOFF", nil
}

// Return whether the source VM is powered off.
func (c *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// Create a snapshot of the source VM.
func (c *Client) CreateSnapshot(vmRef ref.Ref) (string, error) {
	return "", nil
}

// Remove all warm migration snapshots.
func (c *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error {
	return nil
}

// Check if a snapshot is ready to transfer.
func (c *Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (bool, error) {
	return true, nil
}

// Set DataVolume checkpoints.
func (c *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error) {
	return nil
}

// Close connections to the provider API.
func (c *Client) Close() {
}

func (c *Client) Finalize(vms []*planapi.VMStatus, migrationName string) {
	for _, vm := range vms {
		vmModel := &model.VM{}
		err := c.Source.Inventory.Find(vmModel, ref.Ref{ID: vm.Ref.ID})
		if err != nil {
			c.Log.Error(err, "Failed to find vm", "vm", vm.Name)
			return
		}

		for _, av := range vmModel.AttachedVolumes {
			lookupName := fmt.Sprintf("%s-%s", migrationName, av.ID)
			// In a normal operation the snapshot and volume should already have been removed
			// but they may remain in case of failure or cancellation of the migration

			// Delete snapshot
			snapshot := &model.Snapshot{}
			err := c.Source.Inventory.Find(snapshot, ref.Ref{Name: lookupName})
			if err != nil {
				c.Log.Info("Failed to find snapshot", "snapshot", snapshot.Name)
			} else {
				snapshots.Delete(c.OpenstackClient.BlockStorageService, snapshot.ID)
			}

			// Delete cloned volume
			volume := &model.Volume{}
			err = c.Source.Inventory.Find(volume, ref.Ref{Name: lookupName})
			if err != nil {
				c.Log.Info("Failed to find volume", "volume", volume.Name)
			} else {
				volumes.Delete(c.OpenstackClient.BlockStorageService, volume.ID, volumes.DeleteOpts{Cascade: true})
			}

			// Delete Image
			image := &model.Image{}
			err = c.Source.Inventory.Find(image, ref.Ref{Name: lookupName})
			if err != nil {
				c.Log.Info("Failed to find image", "image", image.Name)
			}

			images.Delete(c.OpenstackClient.ImageService, image.ID)
		}
	}
}
