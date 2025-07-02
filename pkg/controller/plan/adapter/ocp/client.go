package ocp

import (
	"context"
	"time"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	export "kubevirt.io/api/export/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	*plancontext.Context
	sourceClient k8sclient.Client
}

// CheckSnapshotReady implements base.Client
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, string, error) {
	return false, "", nil
}

// CheckSnapshotRemove implements base.Client
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	return false, nil
}

// Close implements base.Client
func (r *Client) Close() {
	// NOOP for OCP
}

// CreateSnapshot implements base.Client
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (string, string, error) {
	return "", "", nil
}

// Remove a VM snapshot. No-op for this provider.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (removeTaskId string, err error) {
	return
}

// Get disk deltas for a VM snapshot. No-op for this provider.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (s map[string]string, err error) {
	return
}

// Finalize implements base.Client
func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {
	for _, vm := range vms {
		vmExport := &export.VirtualMachineExport{ObjectMeta: metav1.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
		}}

		err := r.sourceClient.Delete(context.TODO(), vmExport)
		if err != nil {
			r.Log.Info("Failed to delete VMExport", "VMExport", vmExport, "Error", err)
			continue
		}
	}
}

// PowerOff implements base.Client
func (r *Client) PowerOff(vmRef ref.Ref) error {
	vm := cnv.VirtualMachine{}
	err := r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	if vm.Spec.Running != nil {
		running := false
		vm.Spec.Running = &running
	} else if vm.Spec.RunStrategy != nil {
		runStrategy := cnv.RunStrategyHalted
		vm.Spec.RunStrategy = &runStrategy
	}

	err = r.sourceClient.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerOn implements base.Client
func (r *Client) PowerOn(vmRef ref.Ref) error {
	vm := cnv.VirtualMachine{}
	err := r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		return err
	}

	if vm.Spec.Running != nil {
		running := true
		vm.Spec.Running = &running
	} else if vm.Spec.RunStrategy != nil {
		runStrategy := cnv.RunStrategyAlways
		vm.Spec.RunStrategy = &runStrategy
	}

	err = r.sourceClient.Update(context.Background(), &vm)
	if err != nil {
		return err
	}

	return nil
}

// PowerState implements base.Client
func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	vm := cnv.VirtualMachine{}
	err := r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return planapi.VMPowerStateUnknown, err
	}

	if (vm.Spec.Running != nil && *vm.Spec.Running) ||
		(vm.Spec.RunStrategy != nil && *vm.Spec.RunStrategy == cnv.RunStrategyAlways) {
		return planapi.VMPowerStateOn, nil
	}
	return planapi.VMPowerStateOff, nil
}

// PoweredOff implements base.Client
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	vm := cnv.VirtualMachine{}
	err := r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, &vm)
	if err != nil {
		err = liberr.Wrap(err)
		return false, err
	}

	if (vm.Spec.Running != nil && *vm.Spec.Running) ||
		(vm.Spec.RunStrategy != nil && *vm.Spec.RunStrategy == cnv.RunStrategyAlways) {
		return false, nil
	}

	return true, nil
}

// SetCheckpoints implements base.Client
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) (err error) {
	return nil
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}

// PreTransferActions implements base.Builder
func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	apiGroup := cnv.GroupVersion.Group

	// Check if VM export exists
	vmExport := &export.VirtualMachineExport{}
	err = r.sourceClient.Get(context.Background(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vmExport)

	if err != nil {
		if !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to get VM-export.", "vm", vmRef.Name)
			return true, liberr.Wrap(err)
		}

		var tokenTTLDuration *metav1.Duration
		if settings.Settings.CDIExportTokenTTL > 0 {
			tokenTTLDuration = &metav1.Duration{Duration: time.Duration(settings.Settings.CDIExportTokenTTL) * time.Minute}
		}

		// Create VM export
		vmExport = &export.VirtualMachineExport{
			TypeMeta: metav1.TypeMeta{
				Kind:       "VirtualMachineExport",
				APIVersion: "kubevirt.io/v1alpha3",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmRef.Name,
				Namespace: vmRef.Namespace,
			},
			Spec: export.VirtualMachineExportSpec{
				TTLDuration: tokenTTLDuration,
				Source: core.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "VirtualMachine",
					Name:     vmRef.Name,
				},
			},
		}

		err = r.sourceClient.Create(context.Background(), vmExport, &k8sclient.CreateOptions{})
		if err != nil {
			return true, liberr.Wrap(err)
		}
	}
	if vmExport.Status != nil && vmExport.Status.Phase == export.Ready {
		r.Log.Info("VM-export is ready.", "vm", vmRef.Name)
		return true, nil
	}

	r.Log.Info("Waiting for VM-export to be ready...", "vm", vmRef.Name)
	return false, nil
}
