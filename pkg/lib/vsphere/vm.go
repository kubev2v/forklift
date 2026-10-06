package vsphere

import (
	"context"
	"fmt"
	"net"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMRef holds a located virtual machine or template.
type VMRef struct {
	Name  string
	Moref string
	VM    *object.VirtualMachine
}

// GuestIP is a routable guest IP from VMware Tools.
type GuestIP struct {
	Network string
	MAC     string
	IP      string
}

// FindVM locates a VM or template by folder+name.
func (s *Session) FindVM(ctx context.Context, folderPath, name string, asTemplate bool) (*VMRef, error) {
	path := strings.TrimSpace(folderPath)
	lookup := name
	if path != "" {
		lookup = path + "/" + name
	}
	vm, err := s.Finder.VirtualMachine(ctx, lookup)
	if err != nil {
		return nil, err
	}
	var o mo.VirtualMachine
	if err = vm.Properties(ctx, vm.Reference(), []string{"config.template", "name"}, &o); err != nil {
		return nil, err
	}
	isTemplate := o.Config != nil && o.Config.Template
	if asTemplate && !isTemplate {
		return nil, fmt.Errorf("%q is not a template", name)
	}
	if !asTemplate && isTemplate {
		return nil, fmt.Errorf("%q is a template, expected VM", name)
	}
	return &VMRef{Name: o.Name, Moref: vm.Reference().Value, VM: vm}, nil
}

// DestroyIfExists removes VMs matching name under optional folderPath.
func (s *Session) DestroyIfExists(ctx context.Context, folderPath, name string) error {
	patterns := []string{name, "*/" + name}
	if path := strings.TrimSpace(folderPath); path != "" {
		patterns = append(patterns, path+"/"+name)
	}
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		vms, err := s.Finder.VirtualMachineList(ctx, pattern)
		if err != nil {
			continue
		}
		for _, vm := range vms {
			id := vm.Reference().Value
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			if err := Destroy(ctx, vm); err != nil {
				return err
			}
		}
	}
	return nil
}

// Destroy powers off if needed and destroys a VM/template.
func Destroy(ctx context.Context, vm *object.VirtualMachine) error {
	state, err := vm.PowerState(ctx)
	if err == nil && state == types.VirtualMachinePowerStatePoweredOn {
		_, _ = vm.PowerOff(ctx)
	}
	_, err = vm.Destroy(ctx)
	return err
}

// PowerOff hard-powers off a VM. Already-off or missing VMs are no-ops.
func PowerOff(ctx context.Context, vm *object.VirtualMachine) (*object.Task, error) {
	state, err := vm.PowerState(ctx)
	if err != nil {
		if fault.Is(err, &types.ManagedObjectNotFound{}) {
			return nil, nil //nolint:nilnil // no task: VM is already gone
		}
		return nil, liberr.Wrap(err, "vm", vm.Reference().Value)
	}
	if state == types.VirtualMachinePowerStatePoweredOff {
		return nil, nil //nolint:nilnil // no task: already off
	}
	task, err := vm.PowerOff(ctx)
	if err != nil {
		if fault.Is(err, &types.ManagedObjectNotFound{}) || fault.Is(err, &types.InvalidPowerState{}) {
			return nil, nil //nolint:nilnil // no task: gone or already off
		}
		return nil, liberr.Wrap(err, "vm", vm.Reference().Value)
	}
	return task, nil
}

// DestroyVM destroys a VM shell. Missing VMs are a no-op.
func DestroyVM(ctx context.Context, vm *object.VirtualMachine) (*object.Task, error) {
	task, err := vm.Destroy(ctx)
	if err != nil {
		if fault.Is(err, &types.ManagedObjectNotFound{}) {
			return nil, nil //nolint:nilnil // no task: VM is already gone
		}
		return nil, liberr.Wrap(err, "vm", vm.Reference().Value)
	}
	return task, nil
}

// GetTaskInfo resolves a task moRef into TaskInfo.
func GetTaskInfo(ctx context.Context, client *vim25.Client, task string) (*types.TaskInfo, error) {
	t := object.NewTask(client, types.ManagedObjectReference{Type: "Task", Value: task})
	var managedTask mo.Task
	if err := t.Properties(ctx, t.Reference(), []string{"info"}, &managedTask); err != nil {
		return nil, liberr.Wrap(err)
	}
	return &managedTask.Info, nil
}

// GuestAddresses returns routable IPs from guest.net.
func GuestAddresses(ctx context.Context, vm *object.VirtualMachine) ([]GuestIP, error) {
	var managedVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"guest.net"}, &managedVM); err != nil {
		return nil, liberr.Wrap(err, "vm", vm.Reference().Value)
	}
	if managedVM.Guest == nil {
		return nil, nil
	}
	var addresses []GuestIP
	for _, nic := range managedVM.Guest.Net {
		if nic.IpConfig == nil {
			continue
		}
		for _, ip := range nic.IpConfig.IpAddress {
			parsed := net.ParseIP(ip.IpAddress)
			if parsed == nil || parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() ||
				parsed.IsLoopback() || parsed.IsUnspecified() {
				continue
			}
			addresses = append(addresses, GuestIP{
				Network: nic.Network,
				MAC:     strings.ToLower(nic.MacAddress),
				IP:      ip.IpAddress,
			})
		}
	}
	return addresses, nil
}
