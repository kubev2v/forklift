package hyperv

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
)

// stubInventory returns pre-configured VMs and Hosts for testing.
type stubInventory struct {
	base.Client
	vms   map[string]*hyperv.VM
	hosts map[string]*hyperv.Host
}

func (s *stubInventory) Find(resource interface{}, r base.Ref) error {
	switch res := resource.(type) {
	case *hyperv.VM:
		vm, ok := s.vms[r.ID]
		if !ok {
			return base.NotFoundError{Ref: r}
		}
		*res = *vm
	case *hyperv.Host:
		host, ok := s.hosts[r.ID]
		if !ok {
			return base.NotFoundError{Ref: r}
		}
		*res = *host
	}
	return nil
}

func newClusterProvider() *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		Spec: api.ProviderSpec{
			Type: &pt,
			Settings: map[string]string{
				api.ManagementType: api.HyperVCluster,
			},
		},
	}
}

func newStandaloneProvider() *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		Spec: api.ProviderSpec{
			Type: &pt,
		},
	}
}

func TestMaintenanceMode_StandaloneAlwaysOK(t *testing.T) {
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider: newStandaloneProvider(),
			},
		},
	}

	ok, err := v.MaintenanceMode(ref.Ref{ID: "any-vm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("standalone mode should always return ok=true")
	}
}

func makeVM(id, host string) *hyperv.VM {
	vm := &hyperv.VM{Host: host}
	vm.ID = id
	return vm
}

func makeHost(id, state string) *hyperv.Host {
	h := &hyperv.Host{State: state}
	h.ID = id
	return h
}

func TestMaintenanceMode_ClusterHostUp(t *testing.T) {
	inv := &stubInventory{
		vms:   map[string]*hyperv.VM{"vm-1": makeVM("vm-1", "node-1")},
		hosts: map[string]*hyperv.Host{"node-1": makeHost("node-1", model.NodeStateUp)},
	}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	ok, err := v.MaintenanceMode(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true when host state is Up")
	}
}

func TestMaintenanceMode_ClusterHostPaused(t *testing.T) {
	inv := &stubInventory{
		vms:   map[string]*hyperv.VM{"vm-1": makeVM("vm-1", "node-1")},
		hosts: map[string]*hyperv.Host{"node-1": makeHost("node-1", model.NodeStatePaused)},
	}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	ok, err := v.MaintenanceMode(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when host state is Paused")
	}
}

func TestMaintenanceMode_ClusterHostDown(t *testing.T) {
	inv := &stubInventory{
		vms:   map[string]*hyperv.VM{"vm-1": makeVM("vm-1", "node-1")},
		hosts: map[string]*hyperv.Host{"node-1": makeHost("node-1", model.NodeStateDown)},
	}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	ok, err := v.MaintenanceMode(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when host state is Down")
	}
}

func TestMaintenanceMode_ClusterVMNoHost(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makeVM("vm-1", "")},
	}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	ok, err := v.MaintenanceMode(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true when VM has no host (unregistered cluster VM)")
	}
}

func TestMaintenanceMode_ClusterHostNotFound(t *testing.T) {
	inv := &stubInventory{
		vms: map[string]*hyperv.VM{"vm-1": makeVM("vm-1", "missing-node")},
	}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	_, err := v.MaintenanceMode(ref.Ref{ID: "vm-1"})
	if err == nil {
		t.Error("expected error when host lookup fails")
	}
}

func TestMaintenanceMode_VMNotFound(t *testing.T) {
	inv := &stubInventory{}
	v := &Validator{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider:  newClusterProvider(),
				Inventory: inv,
			},
		},
	}

	_, err := v.MaintenanceMode(ref.Ref{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error when VM lookup fails")
	}
}
