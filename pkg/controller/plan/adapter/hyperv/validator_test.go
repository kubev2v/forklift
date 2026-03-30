package hyperv

import (
	"errors"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var errNotImplemented = errors.New("not implemented")

type mockHypervInventory struct {
	vm model.VM
}

func (m *mockHypervInventory) Find(resource interface{}, ref ref.Ref) error {
	switch res := resource.(type) {
	case *model.VM:
		if ref.Name == "missing" {
			return webbase.NotFoundError{}
		}
		*res = m.vm
	}
	return nil
}

func (m *mockHypervInventory) Finder() web.Finder {
	return nil
}

func (m *mockHypervInventory) Get(resource interface{}, id string) error {
	return nil
}

func (m *mockHypervInventory) Host(_ *ref.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

func (m *mockHypervInventory) List(list interface{}, _ ...web.Param) error {
	return nil
}

func (m *mockHypervInventory) Network(_ *ref.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

func (m *mockHypervInventory) Storage(_ *ref.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

func (m *mockHypervInventory) VM(_ *ref.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

func (m *mockHypervInventory) Watch(_ interface{}, _ web.EventHandler) (*web.Watch, error) {
	return nil, errNotImplemented
}

func (m *mockHypervInventory) Workload(_ *ref.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

var _ = Describe("HyperV validator", func() {
	Describe("StaticIPs", func() {
		buildValidator := func(plan *v1beta1.Plan, vm model.VM) *Validator {
			return &Validator{
				Context: &plancontext.Context{
					Plan: plan,
					Source: plancontext.Source{
						Inventory: &mockHypervInventory{vm: vm},
					},
				},
			}
		}

		It("should skip when plan-level preserveStaticIPs is false and no per-VM override", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = false
			plan.Spec.VMs = []planapi.VM{
				{Ref: ref.Ref{ID: "vm-1", Name: "test-vm"}},
			}
			vm := model.VM{
				VM1: model.VM1{VM0: model.Resource{ID: "vm-1", Name: "test-vm"}},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should validate when plan-level is true", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{Ref: ref.Ref{ID: "vm-1", Name: "test-vm"}},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5D:01:02:03"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should skip when per-VM override is false even if plan is true", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:               ref.Ref{ID: "vm-1", Name: "test-vm"},
					PreserveStaticIPs: ptr.To(false),
				},
			}
			vm := model.VM{
				VM1: model.VM1{VM0: model.Resource{ID: "vm-1", Name: "test-vm"}},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should validate when per-VM override is true even if plan is false", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = false
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:               ref.Ref{ID: "vm-1", Name: "test-vm"},
					PreserveStaticIPs: ptr.To(true),
				},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5D:01:02:03"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should pass when excluded NIC has no matching vNIC", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:                      ref.Ref{ID: "vm-1", Name: "test-vm"},
					ExcludeNICsFromStaticIPs: []string{"00:15:5D:01:02:03"},
				},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5D:01:02:04"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10"},
					{MAC: "00:15:5D:01:02:04", IP: "192.168.1.11"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when non-excluded NIC has no matching vNIC", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:                      ref.Ref{ID: "vm-1", Name: "test-vm"},
					ExcludeNICsFromStaticIPs: []string{"00:15:5D:01:02:04"},
				},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5D:01:02:04"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10"},
					{MAC: "00:15:5D:01:02:04", IP: "192.168.1.11"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should pass when all guest networks are excluded", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:                      ref.Ref{ID: "vm-1", Name: "test-vm"},
					ExcludeNICsFromStaticIPs: []string{"00:15:5D:01:02:03"},
				},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5D:01:02:03"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5D:01:02:03", IP: "192.168.1.10"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should fail when no guest networks and nothing excluded", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{Ref: ref.Ref{ID: "vm-1", Name: "test-vm"}},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0:  model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{{MAC: "00:15:5D:01:02:03"}},
				},
				GuestNetworks: []hyperv.GuestNetwork{},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should handle case-insensitive MAC exclusion with hyphen separator", func() {
			plan := createTestPlan()
			plan.Spec.PreserveStaticIPs = true
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:                      ref.Ref{ID: "vm-1", Name: "test-vm"},
					ExcludeNICsFromStaticIPs: []string{"00-15-5D-01-02-03"},
				},
			}
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.Resource{ID: "vm-1", Name: "test-vm"},
					NICs: []hyperv.NIC{
						{MAC: "00:15:5d:01:02:04"},
					},
				},
				GuestNetworks: []hyperv.GuestNetwork{
					{MAC: "00:15:5d:01:02:03", IP: "192.168.1.10"},
					{MAC: "00:15:5d:01:02:04", IP: "192.168.1.11"},
				},
			}
			v := buildValidator(plan, vm)
			ok, err := v.StaticIPs(ref.Ref{ID: "vm-1", Name: "test-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})
