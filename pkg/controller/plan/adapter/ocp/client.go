package ocp

import (
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Client struct {
	*plancontext.Context
}

// CheckSnapshotReady implements base.Client
func (Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (bool, error) {
	return false, nil
}

// Close implements base.Client
func (Client) Close() {
}

// CreateSnapshot implements base.Client
func (Client) CreateSnapshot(vmRef ref.Ref) (string, error) {
	return "", nil
}

// Finalize implements base.Client
func (Client) Finalize(vms []*planapi.VMStatus, planName string) {
}

// PowerOff implements base.Client
func (Client) PowerOff(vmRef ref.Ref) error {
	return nil
}

// PowerOn implements base.Client
func (Client) PowerOn(vmRef ref.Ref) error {
	return nil
}

// PowerState implements base.Client
func (Client) PowerState(vmRef ref.Ref) (string, error) {
	return "", nil
}

// PoweredOff implements base.Client
func (Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	return false, nil
}

// RemoveSnapshots implements base.Client
func (Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error {
	return nil
}

// SetCheckpoints implements base.Client
func (Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error) {
	return nil
}
