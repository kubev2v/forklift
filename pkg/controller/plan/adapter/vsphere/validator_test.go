//nolint:nilnil
package vsphere

import (
	"errors"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrNotImplemented = errors.New("not implemented")

// Mock inventory struct and methods for testing
type mockInventory struct {
	ds model.Datastore
	vm model.VM
}

func (m *mockInventory) Find(resource interface{}, ref ref.Ref) error {
	switch res := resource.(type) {
	case *model.Datastore:
		*res = m.ds
	case *model.Workload:
		*res = model.Workload{VM: m.vm}
		if ref.Name == "full_guest_network" {
			res.VM.GuestNetworks = append(res.VM.GuestNetworks, vsphere.GuestNetwork{MAC: "mac2"})
		}
		if ref.Name == "not_windows_guest" {
			res.VM.GuestID = "rhel8_64Guest"
		}
		if ref.Name == "missing_from_inventory" {
			return base.NotFoundError{}
		}
	case *model.VM:
		*res = m.vm
		if ref.Name == "missing_from_inventory" {
			return base.NotFoundError{}
		}
		if ref.Name == "empty_disk_vm" {
			res.VM1.Disks = []vsphere.Disk{}
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
	return nil, ErrNotImplemented

}

func (m *mockInventory) Storage(ref *ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}

func (m *mockInventory) VM(ref *ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}

func (m *mockInventory) Watch(resource interface{}, h web.EventHandler) (*web.Watch, error) {
	return nil, ErrNotImplemented
}

func (m *mockInventory) Workload(ref *ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}

var _ = Describe("vsphere validation tests", func() {
	Describe("validateStaticIPs", func() {
		DescribeTable("should validate Static IPs correctly",
			func(vmName string, staticIPs, shouldError bool) {
				plan := createPlan()
				ctx := plancontext.Context{
					Plan: plan,
					Source: plancontext.Source{
						Provider: createProvider(),
						Inventory: &mockInventory{
							vm: model.VM{
								VM1: model.VM1{
									VM0: model.VM0{
										ID:   "test-vm-id",
										Name: "test",
									},
									PowerState: "poweredOn",
								},
								GuestNetworks: []vsphere.GuestNetwork{
									{MAC: "mac1", Origin: "STATIC", IP: "192.168.1.5", PrefixLength: 24},
									{MAC: "mac2", Origin: "DHCP", IP: ""},
								},
								NICs: []vsphere.NIC{
									{MAC: "mac1"},
									{MAC: "mac2"},
								},
							},
						}},
				}
				plan.Spec.PreserveStaticIPs = staticIPs
				validator := &Validator{
					Context: &ctx,
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
			Entry("when the vm doesn't exist", "missing_from_inventory", true, true),
		)
	})

	Describe("extractDiskFileName", func() {
		DescribeTable("should extract filename from vSphere disk path",
			func(input, expected string) {
				result := extractDiskFileName(input)
				Expect(result).To(Equal(expected))
			},

			// Standard vSphere disk paths
			Entry("datastore with folder", "[datastore1] folder/vm-disk.vmdk", "vm-disk.vmdk"),
			Entry("datastore without folder", "[datastore1] vm-disk.vmdk", "vm-disk.vmdk"),
			Entry("nested folders", "[datastore1] folder/subfolder/vm-disk.vmdk", "vm-disk.vmdk"),
			Entry("windows-style paths", "[datastore1] folder\\vm-disk.vmdk", "vm-disk.vmdk"),
			Entry("mixed separators", "[datastore1] folder\\subfolder/vm-disk.vmdk", "vm-disk.vmdk"),

			// Edge cases
			Entry("just filename", "vm-disk.vmdk", "vm-disk.vmdk"),
			Entry("empty string", "", ""),
			Entry("path ending with separator", "[datastore1] folder/", ""),
			Entry("no separators", "vm-disk.vmdk", "vm-disk.vmdk"),

			// Real-world examples
			Entry("typical vSphere path", "[datastore1] VMs/test-vm/test-vm.vmdk", "test-vm.vmdk"),
			Entry("shared disk path", "[shared-storage] shared/shared-disk.vmdk", "shared-disk.vmdk"),
		)
	})

	Describe("PVCNameTemplate", func() {
		DescribeTable("should validate PVC name templates correctly",
			func(template, vmName string, shouldPass bool, errorSubstring string) {
				plan := createPlan()
				plan.Spec.PVCNameTemplate = template
				plan.Name = "test-plan"
				ctx := plancontext.Context{
					Plan: plan,
					Source: plancontext.Source{
						Inventory: &mockInventory{
							vm: model.VM{
								VM1: model.VM1{
									VM0: model.VM0{
										ID:   "test-vm-id",
										Name: "test",
									},
									Disks: []vsphere.Disk{
										{
											File:           "[datastore1] VMs/test-vm-test-vm-disk1.vmdk",
											WinDriveLetter: "c",
											Capacity:       1024,
										},
										{
											File:           "[datastore1] VMs/test-vm-test-vm-disk2.vmdk",
											WinDriveLetter: "d",
											Capacity:       2048,
										},
									},
								},
							},
						}},
				}
				validator := &Validator{
					Context: &ctx,
				}

				ok, err := validator.PVCNameTemplate(ref.Ref{Name: vmName, ID: "test-vm-id"}, template)

				if shouldPass {
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(BeTrue())
				} else {
					if errorSubstring != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring(errorSubstring))
					}
					Expect(ok).To(BeFalse())
				}
			},

			// Valid templates
			Entry("valid simple template", "{{.VmName}}-disk-{{.DiskIndex}}", "test", true, ""),
			Entry("valid template with plan name", "{{.PlanName}}-{{.VmName}}-{{.DiskIndex}}", "test", true, ""),
			Entry("valid template with filename", "{{.FileName | trimSuffix \".vmdk\"}}", "test", true, ""),
			Entry("valid template with drive letter", "disk-{{.WinDriveLetter}}", "test", true, ""),
			Entry("valid template with conditional", "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}", "test", true, ""),
			Entry("empty template should pass", "", "test", true, ""),

			// Invalid templates - syntax errors
			Entry("invalid template syntax", "{{.VmName", "test", false, "Invalid template syntax"),
			Entry("invalid template field", "{{.InvalidField}}", "test", false, "can't evaluate field InvalidField"),

			// Invalid templates - empty output
			Entry("template with empty result", "{{ if false }}test{{ end }}", "test", false, "output is empty"),

			// Invalid templates - invalid DNS1123 labels
			Entry("template with invalid characters", "{{.VmName}}_invalid_underscore_{{.DiskIndex}}", "test", false, "invalid k8s label"),
			Entry("template with uppercase", "{{.VmName | upper}}-{{.DiskIndex}}", "test", false, "invalid k8s label"),

			// VM not found error
			Entry("VM not found in inventory", "{{.VmName}}-{{.DiskIndex}}", "missing_from_inventory", false, "not found"),
		)

		It("should handle VM with empty disks", func() {
			plan := createPlan()
			plan.Spec.PVCNameTemplate = "{{.VmName}}-disk-{{.DiskIndex}}"
			ctx := plancontext.Context{
				Plan:   plan,
				Source: plancontext.Source{Inventory: &mockInventory{}},
			}
			validator := &Validator{
				Context: &ctx,
			}

			ok, err := validator.PVCNameTemplate(ref.Ref{Name: "empty_disk_vm", ID: "test-vm-id"}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should use VM-level template over plan-level template", func() {
			plan := createPlan()
			plan.Spec.PVCNameTemplate = "plan-{{.VmName}}-{{.DiskIndex}}"
			// Add VM with specific template
			plan.Spec.VMs = []planapi.VM{
				{
					Ref:             ref.Ref{Name: "test", ID: "test-vm-id"},
					PVCNameTemplate: "vm-{{.VmName}}-{{.DiskIndex}}",
				},
			}

			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{
					Provider:  createProvider(),
					Inventory: &mockInventory{}},
			}
			validator := &Validator{
				Context: &ctx,
			}

			ok, err := validator.PVCNameTemplate(ref.Ref{Name: "test", ID: "test-vm-id"}, "vm-{{.VmName}}-{{.DiskIndex}}")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// VM template validation is now done directly with the passed parameter
		})
	})
})

func createPlan() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-plan-single-vm",
			Namespace: "test",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test",
			VMs:             []planapi.VM{{Ref: ref.Ref{Name: "customer-db-linux-server", ID: "test-vm-id"}}},
			// default by the k8s API
			PVCNameTemplateUseGenerateName: true,
		},
		Referenced: v1beta1.Referenced{
			Provider: struct {
				Source      *v1beta1.Provider
				Destination *v1beta1.Provider
			}{
				Source: &v1beta1.Provider{
					ObjectMeta: metav1.ObjectMeta{Name: "test-vsphere-provider"},
				},
				Destination: &v1beta1.Provider{}},
		},
	}
}

var vsphereProviderType = v1beta1.VSphere

func createProvider() *v1beta1.Provider {
	return &v1beta1.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: "test-provider", Namespace: "test"},
		Spec: v1beta1.ProviderSpec{
			Type:     &vsphereProviderType,
			URL:      "https://127.0.0.1:443/sdk",
			Secret:   core.ObjectReference{},
			Settings: map[string]string{},
		},
	}
}
