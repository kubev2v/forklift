package client

import (
	"context"
	"strings"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

func (r *Client) DeallocateVM(vmRef ref.Ref) error {
	client, err := r.getComputeClient()
	if err != nil {
		return err
	}

	resourceGroup, err := r.vmResourceGroup(vmRef)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), vmOperationTimeout)
	defer cancel()
	poller, err := client.BeginDeallocate(ctx, resourceGroup, vmRef.Name, nil)
	if err != nil {
		return liberr.Wrap(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return liberr.Wrap(err)
	}

	log.Info("Azure VM deallocated", "vm", vmRef.Name)
	return nil
}

func (r *Client) IsVMDeallocated(vmRef ref.Ref) (bool, error) {
	state, err := r.PowerState(vmRef)
	if err != nil {
		return false, err
	}
	return state == planapi.VMPowerStateOff, nil
}

func (r *Client) PowerOn(vmRef ref.Ref) error {
	return nil
}

func (r *Client) PowerOff(vmRef ref.Ref) error {
	return r.DeallocateVM(vmRef)
}

func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	client, err := r.getComputeClient()
	if err != nil {
		return planapi.VMPowerStateUnknown, err
	}

	resourceGroup, err := r.vmResourceGroup(vmRef)
	if err != nil {
		return planapi.VMPowerStateUnknown, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), vmReadTimeout)
	defer cancel()
	result, err := client.InstanceView(ctx, resourceGroup, vmRef.Name, nil)
	if err != nil {
		return planapi.VMPowerStateUnknown, liberr.Wrap(err)
	}

	for _, status := range result.Statuses {
		if status.Code == nil {
			continue
		}
		code := *status.Code
		switch {
		case code == "PowerState/running":
			return planapi.VMPowerStateOn, nil
		case code == "PowerState/deallocated" || code == "PowerState/stopped":
			return planapi.VMPowerStateOff, nil
		case strings.HasPrefix(code, "PowerState/"):
			return planapi.VMPowerStateOff, nil
		}
	}

	return planapi.VMPowerStateUnknown, nil
}

func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	return r.IsVMDeallocated(vmRef)
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (bool, error) {
	isStopped, err := r.PoweredOff(vmRef)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if isStopped {
		return true, nil
	}

	if err := r.PowerOff(vmRef); err != nil {
		return false, liberr.Wrap(err)
	}
	return r.PoweredOff(vmRef)
}

func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {
	for _, vm := range vms {
		if err := r.DeleteSnapshots(vm.Ref); err != nil {
			log.Error(err, "Failed to delete Azure snapshots during finalize", "vm", vm.Name)
		}
		if err := r.DeletePreSnapshots(vm.Ref); err != nil {
			log.Error(err, "Failed to delete Azure pre-snapshots during finalize", "vm", vm.Name)
		}
	}
}
