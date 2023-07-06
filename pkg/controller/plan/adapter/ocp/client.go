package ocp

import (
	"context"

	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	export "kubevirt.io/api/export/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func (r Client) PowerOff(vmRef ref.Ref) error {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	// TODO: is vm.Spec.RunStrategy = &runStrategyHalted also needed?
	running := false
	vm.Spec.Running = &running
	err = r.Client.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerOn implements base.Client
func (r Client) PowerOn(vmRef ref.Ref) error {
	r.Log.Info("Benny powerOn")
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	running := true
	vm.Spec.Running = &running
	err = r.Client.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerState implements base.Client
func (r Client) PowerState(vmRef ref.Ref) (string, error) {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return "", err
	}

	if vm.Spec.Running != nil && *vm.Spec.Running {
		return "On", nil
	}

	return "Off", nil
}

// PoweredOff implements base.Client
func (r Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	vm := cnv.VirtualMachine{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return false, err
	}

	if vm.Spec.Running != nil && *vm.Spec.Running {
		return false, nil
	}

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
