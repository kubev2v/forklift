package openstack

import (
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/openstack"
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
