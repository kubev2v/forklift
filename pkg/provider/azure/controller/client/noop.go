package client

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func (r *Client) PowerOn(vmRef ref.Ref) error  { return nil }
func (r *Client) PowerOff(vmRef ref.Ref) error { return nil }
func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	return planapi.VMPowerStateUnknown, nil
}
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error)            { return true, nil }
func (r *Client) PreTransferActions(vmRef ref.Ref) (bool, error)    { return true, nil }
func (r *Client) DetachDisks(vmRef ref.Ref) error                   { return nil }
func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {}
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (string, string, error) {
	return "", "", nil
}
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (string, error) {
	return "", nil
}
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, string, error) {
	return true, "", nil
}
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	return true, nil
}
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) error {
	return nil
}
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (map[string]string, error) {
	return nil, nil
}

var _ base.Client = &Client{}
