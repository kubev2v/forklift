package ocp

import (
	"context"

	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	export "kubevirt.io/api/export/v1alpha1"
	kubecli "kubevirt.io/client-go/kubecli"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Client struct {
	*plancontext.Context
	kubecli.KubevirtClient
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
func (r Client) Finalize(vms []*planapi.VMStatus, planName string) {
	for _, vm := range vms {
		vmExport := &export.VirtualMachineExport{ObjectMeta: v1.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
		}}

		err := r.Client.Delete(context.TODO(), vmExport)
		if err != nil {
			r.Log.Info("Failed to delete VMExport", "VMExport", vmExport, "Error", err)
			continue
		}
	}
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
	return true, nil
}

// RemoveSnapshots implements base.Client
func (Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error {
	return nil
}

// SetCheckpoints implements base.Client
func (Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error) {
	return nil
}
