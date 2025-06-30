//nolint:nilnil
package vsphere

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock inventory struct and methods for testing
type mockInventory struct{}

func (m *mockInventory) Find(resource interface{}, ref ref.Ref) error {
	switch res := resource.(type) {
	case *model.Workload:
		*res = model.Workload{
			VM: model.VM{
				NICs: []vsphere.NIC{
					{MAC: "mac1"},
					{MAC: "mac2"},
				},
				GuestNetworks: []vsphere.GuestNetwork{
					{MAC: "mac1"},
				},
				GuestID: "windows7Guest",
				VM1: model.VM1{
					PowerState: "poweredOn", // default state
				},
			},
		}
		if ref.Name == "full_guest_network" {
			res.VM.GuestNetworks = append(res.VM.GuestNetworks, vsphere.GuestNetwork{MAC: "mac2"})
		}
		if ref.Name == "not_windows_guest" {
			res.VM.GuestID = "rhel8_64Guest"
		}
		if ref.Name == "missing_from_invetory" {
			return base.NotFoundError{}
		}
	}
	return nil
}

func (m *mockInventory) Finder() web.Finder {
	return nil
}

func (m *mockInventory) Get(resource interface{}, id string) error {
	return nil
}

func (m *mockInventory) Host(ref *ref.Ref) (interface{}, error) {
	return nil, nil
}

func (m *mockInventory) List(list interface{}, param ...web.Param) error {
	return nil
}

func (m *mockInventory) Network(ref *ref.Ref) (interface{}, error) {
	return nil, nil
}

func (m *mockInventory) Storage(ref *ref.Ref) (interface{}, error) {
	return nil, nil
}

func (m *mockInventory) VM(ref *ref.Ref) (interface{}, error) {
	return nil, nil
}

func (m *mockInventory) Watch(resource interface{}, h web.EventHandler) (*web.Watch, error) {
	return nil, nil
}

func (m *mockInventory) Workload(ref *ref.Ref) (interface{}, error) {
	return nil, nil
}

var _ = Describe("vsphere validation tests", func() {
	Describe("validateStaticIPs", func() {
		DescribeTable("should validate Static IPs correctly",
			func(vmName string, staticIPs, shouldError bool) {
				plan := createPlan()
				plan.Spec.PreserveStaticIPs = staticIPs
				validator := &Validator{
					plan:      plan,
					inventory: &mockInventory{},
				}
				ok, err := validator.StaticIPs(ref.Ref{Name: vmName})
				if shouldError {
					Expect(ok).To(BeFalse())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(BeTrue())
				}
			},

			// Directly declare entries here
			Entry("when the vm doesn't have static ips, and the plan set with static ip", "test", true, false),
			Entry("when the vm doesn't have static ips, and the plan set without static ip", "test", false, false),
			Entry("when the vm have static ips, and the plan set with static ip", "full_guest_network", true, false),
			Entry("when the vm have static ips, and the plan set without static ip", "test", false, false),
			Entry("when the vm doesn't have static ips, and the plan set without static ip, vm is non-windows", "not_windows_guest", false, false),
			Entry("when the vm doesn't have static ips, and the plan set with static ip, vm is non-windows", "not_windows_guest", true, false),
			Entry("when the vm doesn't exist", "missing_from_invetory", true, true),
		)
	})
})

func createPlan() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test",
			VMs:             []plan.VM{{Ref: ref.Ref{Name: "test"}}},
		},
	}
}
