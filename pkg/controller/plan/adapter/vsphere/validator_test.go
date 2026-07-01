//nolint:nilnil
package vsphere

import (
	"context"
	"errors"
	"fmt"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	calicoclient "github.com/kubev2v/forklift/pkg/lib/client/calico"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var ErrNotImplemented = errors.New("not implemented")

// Mock inventory struct and methods for testing
type mockInventory struct {
	ds       model.Datastore
	vm       model.VM
	networks map[string]model.Network // keyed by ID
}

// defaultVM returns a VM with sensible defaults for testing
func defaultVM() model.VM {
	return model.VM{
		ToolsStatus:        ToolsOk,           // default: tools installed
		ToolsRunningStatus: GuestToolsRunning, // default: tools running
		ToolsVersionStatus: GuestToolsCurrent, // default: tools current
		VM1: model.VM1{
			VM0: model.VM0{
				ID:   "test-vm-id",
				Name: "test-vm",
			},
			PowerState: "poweredOn", // default state
			Disks: []vsphere.Disk{
				{
					File:           "[datastore1] VMs/test-vm/test-vm-disk1.vmdk",
					WinDriveLetter: "c",
					Capacity:       1024,
				},
				{
					File:           "[datastore1] VMs/test-vm/test-vm-disk2.vmdk",
					WinDriveLetter: "d",
					Capacity:       2048,
				},
			},
		},
	}
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
		if ref.Name == "nics_no_guest_networks" {
			res.VM.GuestNetworks = nil
		}
		if ref.Name == "missing_from_inventory" {
			return base.NotFoundError{}
		}
	case *model.Network:
		if m.networks != nil {
			if net, ok := m.networks[ref.ID]; ok {
				*res = net
				return nil
			}
		}
		return base.NotFoundError{}
	case *model.VM:
		if ref.Name == "missing_from_inventory" {
			return base.NotFoundError{}
		}
		// Use m.vm if set, otherwise use default VM
		if m.vm.ID != "" {
			*res = m.vm
		} else {
			*res = defaultVM()
		}
		if ref.Name == "empty_disk_vm" {
			res.VM1.Disks = []vsphere.Disk{}
		}
		// Test cases for GuestToolsInstalled
		switch ref.Name {
		case "tools_not_installed":
			res.ToolsStatus = ToolsNotInstalled
		case "tools_not_running":
			res.ToolsStatus = ToolsOk
			res.ToolsRunningStatus = GuestToolsNotRunning
		case "tools_status_unknown":
			res.ToolsStatus = "" // Empty status (encrypted VM scenario)
		case "tools_status_null":
			res.ToolsStatus = "null" // Null status (encrypted VM scenario)
		case "tools_status_nil":
			res.ToolsStatus = "<nil>" // fmt.Sprint(nil) result (encrypted VM scenario)
		case "vm_powered_off":
			res.PowerState = "poweredOff"
			res.ToolsStatus = ToolsNotInstalled // Should be ignored when powered off
		case "tools_unmanaged":
			res.ToolsVersionStatus = GuestToolsUnmanaged
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
					Source: plancontext.Source{Inventory: &mockInventory{
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
			Entry("when the vm has NICs but no guest networks, and the plan set with static ip", "nics_no_guest_networks", true, true),
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
				Plan:   plan,
				Source: plancontext.Source{Inventory: &mockInventory{}},
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

	Describe("GuestToolsInstalled", func() {
		DescribeTable("should validate VMware Tools status correctly",
			func(vmName string, expectedOk bool, shouldError bool) {
				provider := &v1beta1.Provider{Spec: v1beta1.ProviderSpec{Type: &[]v1beta1.ProviderType{v1beta1.VSphere}[0]}}
				validator := &Validator{
					Context: &plancontext.Context{},
				}
				validator.Source = plancontext.Source{Provider: provider}
				validator.Source.Inventory = &mockInventory{}

				vmRef := ref.Ref{Name: vmName}
				ok, err := validator.GuestToolsInstalled(vmRef)

				if shouldError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(Equal(expectedOk))
				}
			},

			// Success cases
			Entry("when VMware Tools are installed and running", "default_vm", true, false),

			// Critical cases - powered on VMs with tools issues
			Entry("when VMware Tools are not installed", "tools_not_installed", false, false),
			Entry("when VMware Tools are not running", "tools_not_running", false, false),

			// Unmanaged tools (open-vm-tools) should pass validation
			Entry("when VMware Tools are unmanaged (open-vm-tools)", "tools_unmanaged", true, false),

			// Encrypted/unknown tools status must block when VM is powered on
			Entry("when VMware Tools status is empty (encrypted VM) -> block", "tools_status_unknown", false, false),
			Entry("when VMware Tools status is null (encrypted VM) -> block", "tools_status_null", false, false),
			Entry("when VMware Tools status is <nil> (encrypted VM) -> block", "tools_status_nil", false, false),

			// Powered off VMs should pass validation
			Entry("when VM is powered off (tools status ignored)", "vm_powered_off", true, false),

			// Error cases
			Entry("when VM is missing from inventory", "missing_from_inventory", false, true),
		)
	})

	Describe("NICNetworkRefs + ValidateNetworkDuplicates", func() {
		It("should return no refs for VM with no NICs", func() {
			plan := createPlan()
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{Inventory: &mockInventory{
					vm: model.VM{
						VM1: model.VM1{VM0: model.VM0{ID: "vm-1", Name: "test"}},
					},
				}},
			}
			validator := &Validator{Context: &ctx}
			nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test"})
			Expect(err).NotTo(HaveOccurred())
			Expect(nicRefs).To(BeEmpty())
		})

		It("should return ok when no duplicate NADs exist", func() {
			plan := createPlan()
			plan.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}},
							Destination: v1beta1.DestinationNetwork{
								Type:      "multus",
								Namespace: "ns1",
								Name:      "nad-a",
							},
						},
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}},
							Destination: v1beta1.DestinationNetwork{
								Type:      "multus",
								Namespace: "ns1",
								Name:      "nad-b",
							},
						},
					},
				},
			}
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{Inventory: &mockInventory{
					vm: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1", Name: "test"},
						},
						NICs: []vsphere.NIC{
							{Network: vsphere.Ref{ID: "net-1"}, MAC: "aa:bb:cc:dd:ee:01"},
							{Network: vsphere.Ref{ID: "net-2"}, MAC: "aa:bb:cc:dd:ee:02"},
						},
					},
				}},
			}
			validator := &Validator{Context: &ctx}
			nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test"})
			Expect(err).NotTo(HaveOccurred())
			foundNadDup, foundPodDup := planbase.ValidateNetworkDuplicates(nicRefs, plan.Map.Network)
			Expect(foundNadDup).To(BeFalse())
			Expect(foundPodDup).To(BeFalse())
		})

		It("should detect duplicate when two NICs on same source network map to same NAD", func() {
			plan := createPlan()
			plan.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}},
							Destination: v1beta1.DestinationNetwork{
								Type:      "multus",
								Namespace: "ns1",
								Name:      "nad-a",
							},
						},
					},
				},
			}
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{Inventory: &mockInventory{
					vm: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1", Name: "test"},
						},
						NICs: []vsphere.NIC{
							{Network: vsphere.Ref{ID: "net-1"}, MAC: "aa:bb:cc:dd:ee:01"},
							{Network: vsphere.Ref{ID: "net-1"}, MAC: "aa:bb:cc:dd:ee:02"},
						},
					},
				}},
			}
			validator := &Validator{Context: &ctx}
			nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test"})
			Expect(err).NotTo(HaveOccurred())
			foundNadDup, _ := planbase.ValidateNetworkDuplicates(nicRefs, plan.Map.Network)
			Expect(foundNadDup).To(BeTrue())
		})

		It("should detect duplicate when two different source networks map to same NAD", func() {
			plan := createPlan()
			plan.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}},
							Destination: v1beta1.DestinationNetwork{
								Type:      "multus",
								Namespace: "ns1",
								Name:      "nad-a",
							},
						},
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}},
							Destination: v1beta1.DestinationNetwork{
								Type:      "multus",
								Namespace: "ns1",
								Name:      "nad-a",
							},
						},
					},
				},
			}
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{Inventory: &mockInventory{
					vm: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1", Name: "test"},
						},
						NICs: []vsphere.NIC{
							{Network: vsphere.Ref{ID: "net-1"}, MAC: "aa:bb:cc:dd:ee:01"},
							{Network: vsphere.Ref{ID: "net-2"}, MAC: "aa:bb:cc:dd:ee:02"},
						},
					},
				}},
			}
			validator := &Validator{Context: &ctx}
			nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test"})
			Expect(err).NotTo(HaveOccurred())
			foundNadDup, _ := planbase.ValidateNetworkDuplicates(nicRefs, plan.Map.Network)
			Expect(foundNadDup).To(BeTrue())
		})

		It("should detect multiple pod networks via foundPodDup", func() {
			plan := createPlan()
			plan.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{
						{
							Source:      v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-1"}},
							Destination: v1beta1.DestinationNetwork{Type: "pod"},
						},
						{
							Source:      v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-2"}},
							Destination: v1beta1.DestinationNetwork{Type: "pod"},
						},
						{
							Source:      v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-3"}},
							Destination: v1beta1.DestinationNetwork{Type: "ignored"},
						},
					},
				},
			}
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{Inventory: &mockInventory{
					vm: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1", Name: "test"},
						},
						NICs: []vsphere.NIC{
							{Network: vsphere.Ref{ID: "net-1"}, MAC: "aa:bb:cc:dd:ee:01"},
							{Network: vsphere.Ref{ID: "net-2"}, MAC: "aa:bb:cc:dd:ee:02"},
							{Network: vsphere.Ref{ID: "net-3"}, MAC: "aa:bb:cc:dd:ee:03"},
						},
					},
				}},
			}
			validator := &Validator{Context: &ctx}
			nicRefs, err := validator.NICNetworkRefs(ref.Ref{Name: "test"})
			Expect(err).NotTo(HaveOccurred())
			foundNadDup, foundPodDup := planbase.ValidateNetworkDuplicates(nicRefs, plan.Map.Network)
			Expect(foundNadDup).To(BeFalse())
			Expect(foundPodDup).To(BeTrue()) // two NICs mapped to pod
		})

		It("should return error when VM not found", func() {
			ctx := plancontext.Context{
				Plan:   createPlan(),
				Source: plancontext.Source{Inventory: &mockInventory{}},
			}
			validator := &Validator{Context: &ctx}
			_, err := validator.NICNetworkRefs(ref.Ref{Name: "missing_from_inventory"})
			Expect(err).To(HaveOccurred())
		})
	})

	DescribeTable("ConsolidationNeeded",
		func(consolidationNeeded bool) {
			plan := createPlan()
			ctx := plancontext.Context{
				Plan: plan,
				Source: plancontext.Source{
					Inventory: &mockInventory{
						vm: model.VM{
							VM1: model.VM1{
								VM0: model.VM0{
									ID:   "consolidation-vm-1",
									Name: "consolidation-vm",
								},
							},
							ConsolidationNeeded: consolidationNeeded,
						},
					},
				},
			}
			validator := Validator{Context: &ctx}
			needed, err := validator.ConsolidationNeeded(ref.Ref{Name: "consolidation-vm"})
			Expect(err).NotTo(HaveOccurred())
			Expect(needed).To(Equal(consolidationNeeded))
		},
		Entry("should warn on consolidation needed", true),
		Entry("should not warn when consolidation is not needed", false),
	)

	Describe("Calico Network validation", func() {
		// Reusable identifiers across cases.
		const (
			srcNetID = "src-1"
			nadName  = "calico-nad"
			nadNS    = "workloads"
			netName  = "vlan100"
		)

		makeCalicoNAD := func(vlan int) *k8snet.NetworkAttachmentDefinition {
			cfg := fmt.Sprintf(`{"type":"calico","network":"%s","vlan":%d}`, netName, vlan)
			return &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: nadName, Namespace: nadNS},
				Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: cfg},
			}
		}
		makeNetwork := func(spec map[string]interface{}) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.NetworkGVK)
			u.SetName(netName)
			if spec != nil {
				_ = unstructured.SetNestedField(u.Object, spec, "spec")
			}
			return u
		}
		makeIPPool := func(name, cidr string) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.IPPoolGVK)
			u.SetName(name)
			_ = unstructured.SetNestedField(u.Object, cidr, "spec", "cidr")
			return u
		}
		// L2Bridge spec helpers — VLAN 100 maps to subnet 10.100.0.0/24.
		l2Single := map[string]interface{}{
			"l2Bridge": map[string]interface{}{
				"vlans": []interface{}{
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(100)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
					},
				},
			},
		}
		l2Multi := map[string]interface{}{
			"l2Bridge": map[string]interface{}{
				"vlans": []interface{}{
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(100)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
					},
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(200)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/24"}},
					},
				},
			},
		}

		// setup builds a Validator + fake client. nicIP is the NIC's guest IP,
		// preserveIPs controls Plan.Spec.PreserveStaticIPs.
		setup := func(nicIP string, preserveIPs bool, k8sObjs ...runtime.Object) (*Validator, client.Client, ref.Ref) {
			scheme := runtime.NewScheme()
			_ = k8snet.AddToScheme(scheme)
			scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
			scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(k8sObjs...).Build()

			vm := model.VM{
				VM1:  model.VM1{VM0: model.VM0{ID: "test-vm-id", Name: "test-vm"}},
				NICs: []vsphere.NIC{{Network: vsphere.Ref{ID: srcNetID}, DeviceKey: 4001}},
				GuestNetworks: []vsphere.GuestNetwork{
					{IP: nicIP, DeviceConfigId: 4001},
				},
			}
			inventory := &mockInventory{
				vm: vm,
				networks: map[string]model.Network{
					srcNetID: {Resource: model.Resource{ID: srcNetID}, Variant: vsphere.NetDvPortGroup, Key: srcNetID},
				},
			}
			plan := createPlan()
			plan.Spec.PreserveStaticIPs = preserveIPs
			plan.Referenced.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: srcNetID}},
						Destination: v1beta1.DestinationNetwork{
							Type: planbase.Multus, Namespace: nadNS, Name: nadName,
						},
					}},
				},
			}
			ctx := plancontext.Context{Plan: plan, Source: plancontext.Source{Inventory: inventory}}
			return &Validator{Context: &ctx}, c, ref.Ref{Name: "test-vm-id", ID: "test-vm-id"}
		}

		// nadIssueKinds extracts the set of NAD-issue kinds from a slice.
		nadIssueKinds := func(issues []planbase.CalicoNADIssue) []planbase.CalicoIssueKind {
			out := make([]planbase.CalicoIssueKind, 0, len(issues))
			for _, i := range issues {
				out = append(out, i.Kind)
			}
			return out
		}

		Describe("ValidateCalicoNADs (plan-level)", func() {
			It("returns empty results when NetworkMap is nil", func() {
				v, c, _ := setup("10.100.0.5", true)
				v.Plan.Referenced.Map.Network = nil
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache).NotTo(BeNil())
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("happy path — populates cache, no issues, when Network, VLAN, IPPool all line up", func() {
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.NADs).To(HaveLen(1))
				entry := result.Cache.NADs[k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}]
				Expect(entry).NotTo(BeNil())
				Expect(entry.VLAN.VID).To(Equal(uint16(100)))
				Expect(entry.EligiblePools).To(HaveLen(1))
			})

			It("emits NetworkCRDAbsent when the Network CRD itself is missing", func() {
				// Calico installed (NAD readable, IPPool path would work)
				// but no projectcalico.org/v3 Network CRD on the cluster.
				// The NAD references a Calico Network → can't honour the
				// L2 attach → Critical NetworkCRDAbsent (distinct from a
				// missing CR which would be NetworkNotFound).
				v, _, _ := setup("10.100.0.5", true, makeCalicoNAD(100))
				networkGK := calicoclient.NetworkGVK.GroupKind()
				scheme := runtime.NewScheme()
				_ = k8snet.AddToScheme(scheme)
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
				c := fake.NewClientBuilder().WithScheme(scheme).
					WithRuntimeObjects(makeCalicoNAD(100)).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, inner client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if obj.GetObjectKind().GroupVersionKind() == calicoclient.NetworkGVK {
								return &meta.NoKindMatchError{GroupKind: networkGK}
							}
							// Fall through to the underlying fake client
							// for other GETs (notably the NAD).
							return inner.Get(ctx, key, obj, opts...)
						},
					}).Build()
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:     k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind:    planbase.CalicoIssueNetworkCRDAbsent,
					Network: netName,
					VLAN:    100,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("emits NetworkNotFound when the referenced Network is missing", func() {
				v, c, _ := setup("10.100.0.5", true, makeCalicoNAD(100))
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:     k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind:    planbase.CalicoIssueNetworkNotFound,
					Network: netName,
					VLAN:    100,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("emits NetworkHasNoL2Bridge when the Network exists but has no l2Bridge", func() {
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(map[string]interface{}{}),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueNetworkHasNoL2Bridge))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("emits VLANRequired when the NAD references a Calico Network but omits vlan", func() {
				// A Network reference requires an explicit VLAN. The rejection
				// fires before the Network is fetched, so the Network's own
				// contents (single, multi, or empty vlans) are irrelevant.
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(0), makeNetwork(l2Multi),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANRequired))
			})

			It("emits NetworkHasNoVLANs even when the NAD specifies a vlan ID", func() {
				// Same root cause: Network has no VLAN entries. The NAD's vlan
				// value is moot — there's nothing to match against.
				emptyVLANs := map[string]interface{}{
					"l2Bridge": map[string]interface{}{
						"vlans": []interface{}{},
					},
				}
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(emptyVLANs),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueNetworkHasNoVLANs))
			})

			It("emits VLANNotInNetwork when the NAD vlan ID doesn't match any entry", func() {
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(999), makeNetwork(l2Single),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANNotInNetwork))
			})

			It("emits VLANHasNoIPPool when no IPPool overlaps the VLAN subnet", func() {
				v, c, _ := setup("10.100.0.5", false,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("cluster-default", "10.0.0.0/8"), // pool too large
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANHasNoIPPool))
			})

			It("dedupes the same NAD when referenced by multiple network-map pairs", func() {
				// Same NAD destination referenced by two source networks. Without
				// dedup, ValidateCalicoNADs would emit NetworkNotFound twice and
				// double-fetch the NAD.
				v, c, _ := setup("10.100.0.5", true, makeCalicoNAD(100))
				v.Plan.Referenced.Map.Network.Spec.Map = append(
					v.Plan.Referenced.Map.Network.Spec.Map,
					v1beta1.NetworkPair{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}},
						Destination: v1beta1.DestinationNetwork{
							Type: planbase.Multus, Namespace: nadNS, Name: nadName,
						},
					},
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(HaveLen(1))
				Expect(result.Issues[0].Kind).To(Equal(planbase.CalicoIssueNetworkNotFound))
			})

			It("emits NADUnreadable when the network map references a missing NAD", func() {
				// Network map destination points at a NAD that doesn't exist on
				// the destination cluster. ValidateCalicoNADs must not propagate
				// the NotFound — it should soft-fail and surface a NADUnreadable
				// issue so the plan validation pass can complete.
				v, c, _ := setup("10.100.0.5", true) // no NAD in the client
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:  k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind: planbase.CalicoIssueNADUnreadable,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("emits NADUnreadable when the NAD spec.config is malformed JSON", func() {
				badNAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: nadName, Namespace: nadNS},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: `{not-valid-json`},
				}
				v, c, _ := setup("10.100.0.5", true, badNAD)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:  k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind: planbase.CalicoIssueNADUnreadable,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("skips Multus NADs that aren't Calico-typed", func() {
				// An OVN-K8s overlay NAD shouldn't surface a CalicoNADIssue or
				// occupy a cache slot — it's outside the scope of this validator.
				ovnNAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "ovn-nad", Namespace: nadNS},
					Spec: k8snet.NetworkAttachmentDefinitionSpec{
						Config: `{"type":"ovn-k8s-cni-overlay","name":"my-net"}`,
					},
				}
				v, c, _ := setup("10.100.0.5", true, ovnNAD)
				v.Plan.Referenced.Map.Network.Spec.Map[0].Destination.Name = "ovn-nad"
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("warns when a Calico-typed NAD has no 'network' field (L3 mode, no identity preservation)", func() {
				// type:calico without a network reference is Calico's legacy L3
				// IPAM mode. Forklift's MAC/IP annotation stamping is gated on
				// the presence of a Calico Network reference, so this NAD would
				// silently miss preservation. Warn-class issue surfaces the gap
				// without blocking the migration.
				l3NAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: nadName, Namespace: nadNS},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: `{"type":"calico"}`},
				}
				v, c, _ := setup("10.100.0.5", true, l3NAD)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:  k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind: planbase.CalicoIssueNADMissingNetwork,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})

			It("populates Issues and Warnings independently when both classes coexist", func() {
				// Two NADs in the same NetworkMap: the original (L2 Calico, but
				// the referenced Network CR is absent — Critical NetworkNotFound)
				// and a second L3-mode NAD (Warn NADMissingNetwork). Asserts the
				// dispatcher's independence claim: both slices populated, items
				// disjoint, no cross-contamination between Critical and Warn.
				const (
					l3NADName = "calico-nad-l3"
					l3SrcID   = "src-l3"
				)
				l3NAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: l3NADName, Namespace: nadNS},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: `{"type":"calico"}`},
				}
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), // existing nadName, references missing Network "vlan100"
					l3NAD,
				)
				v.Plan.Referenced.Map.Network.Spec.Map = append(
					v.Plan.Referenced.Map.Network.Spec.Map,
					v1beta1.NetworkPair{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: l3SrcID}},
						Destination: v1beta1.DestinationNetwork{
							Type: planbase.Multus, Namespace: nadNS, Name: l3NADName,
						},
					},
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:     k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind:    planbase.CalicoIssueNetworkNotFound,
					Network: netName,
					VLAN:    100,
				}))
				Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:  k8stypes.NamespacedName{Namespace: nadNS, Name: l3NADName},
					Kind: planbase.CalicoIssueNADMissingNetwork,
				}))
				Expect(result.Cache.NADs).To(BeEmpty())
			})
		})

		Describe("plan-level / per-VM cross-cut", func() {
			It("surviving healthy NAD is still checked per-VM when another NAD in the map is broken", func() {
				// Two NADs in the map: the original (broken — no Network CR) and
				// a second healthy one. The VM has a NIC mapped to the healthy
				// NAD with an out-of-subnet IP. CalicoVMIssues must skip the
				// broken-NAD NIC silently and still emit IPNotInSubnet for the
				// surviving NIC.
				const (
					healthyNADName = "calico-nad-2"
					healthyNetName = "vlan200"
					healthySrcID   = "src-2"
				)
				healthyNAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: healthyNADName, Namespace: nadNS},
					Spec: k8snet.NetworkAttachmentDefinitionSpec{
						Config: fmt.Sprintf(`{"type":"calico","network":"%s","vlan":200}`, healthyNetName),
					},
				}
				healthyNet := &unstructured.Unstructured{}
				healthyNet.SetGroupVersionKind(calicoclient.NetworkGVK)
				healthyNet.SetName(healthyNetName)
				_ = unstructured.SetNestedField(healthyNet.Object, map[string]interface{}{
					"l2Bridge": map[string]interface{}{
						"vlans": []interface{}{
							map[string]interface{}{
								"vlan":    map[string]interface{}{"id": int64(200)},
								"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/24"}},
							},
						},
					},
				}, "spec")

				// NIC 4001 (src-1) maps to the broken NAD; NIC 4002 (src-2) maps
				// to the healthy NAD and carries an out-of-subnet IP.
				v, c, vmRef := setup("10.100.0.5", true,
					makeCalicoNAD(100), // broken — no Network CR
					healthyNAD, healthyNet,
					makeIPPool("vlan200-pool", "10.200.0.0/24"),
				)
				v.Plan.Referenced.Map.Network.Spec.Map = append(
					v.Plan.Referenced.Map.Network.Spec.Map,
					v1beta1.NetworkPair{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: healthySrcID}},
						Destination: v1beta1.DestinationNetwork{
							Type: planbase.Multus, Namespace: nadNS, Name: healthyNADName,
						},
					},
				)
				inv := v.Source.Inventory.(*mockInventory)
				inv.networks[healthySrcID] = model.Network{
					Resource: model.Resource{ID: healthySrcID}, Variant: vsphere.NetDvPortGroup, Key: healthySrcID,
				}
				inv.vm.NICs = append(inv.vm.NICs, vsphere.NIC{Network: vsphere.Ref{ID: healthySrcID}, DeviceKey: 4002})
				inv.vm.GuestNetworks = append(inv.vm.GuestNetworks, vsphere.GuestNetwork{IP: "192.168.1.5", DeviceConfigId: 4002})

				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(HaveLen(1))
				Expect(result.Issues[0].NAD).To(Equal(k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}))
				Expect(result.Cache.NADs).To(HaveLen(1))
				Expect(result.Cache.NADs).To(HaveKey(k8stypes.NamespacedName{Namespace: nadNS, Name: healthyNADName}))

				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoIssue{
					Kind: planbase.CalicoIssueIPNotInSubnet, Network: healthyNetName, VLAN: 200, IP: "192.168.1.5",
				}))
			})
		})

		Describe("CalicoVMIssues (per-VM)", func() {
			It("returns no issues when the cache is nil", func() {
				v, _, vmRef := setup("10.100.0.5", true)
				issues, err := v.CalicoVMIssues(vmRef, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("emits IPNotInSubnet when preserveStaticIPs is on and source IP is outside the VLAN subnet", func() {
				v, c, vmRef := setup("192.168.1.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoIssue{
					Kind: planbase.CalicoIssueIPNotInSubnet, Network: netName, VLAN: 100, IP: "192.168.1.5",
				}))
			})

			It("emits IPNotInIPPool when preserveStaticIPs is on and no eligible pool covers the source IP", func() {
				v, c, vmRef := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-upper", "10.100.0.128/25"), // covers VLAN but not 10.100.0.5
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoIssue{
					Kind: planbase.CalicoIssueIPNotInIPPool, Network: netName, VLAN: 100, IP: "10.100.0.5",
				}))
			})

			It("returns no issues when preserveStaticIPs is false", func() {
				// Source IP would fail subnet check, but preservation is off.
				v, c, vmRef := setup("192.168.1.5", false,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("silently skips NICs whose NAD failed plan-level validation", func() {
				// Network is missing, so ValidateCalicoNADs flags the NAD and
				// leaves the cache empty. CalicoVMIssues must not re-emit per-VM
				// issues for that NAD — the failure is already reported at plan
				// level.
				v, c, vmRef := setup("10.100.0.5", true, makeCalicoNAD(100))
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Cache.NADs).To(BeEmpty())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("emits TooManyIPs when a NIC carries more than one IPv4", func() {
				// Calico's ipAddrs annotation accepts at most one IPv4 per
				// interface; a multi-IPv4 NIC would fail the pod at CNI ADD.
				v, c, vmRef := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24"),
				)
				vm := v.Source.Inventory.(*mockInventory).vm
				vm.GuestNetworks = append(vm.GuestNetworks, vsphere.GuestNetwork{IP: "10.100.0.6", DeviceConfigId: 4001})
				v.Source.Inventory.(*mockInventory).vm = vm

				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(HaveLen(1))
				Expect(issues[0].Kind).To(Equal(planbase.CalicoIssueTooManyIPs))
			})

			It("deduplicates identical per-NIC IP issues", func() {
				// Two NICs on the same source network, two NetworkMap entries each
				// pointing at a distinct NAD; both NADs reference the same Calico
				// Network. The pool gives NIC-0 NAD-A and NIC-1 NAD-B. Both NICs
				// carry the same out-of-subnet IP so both emit identical
				// {Kind, Network, VLAN, IP}; CalicoVMIssues must dedup to one.
				nadAName := "calico-nad-a"
				nadBName := "calico-nad-b"
				cfg := fmt.Sprintf(`{"type":"calico","network":"%s","vlan":100}`, netName)
				nadA := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: nadAName, Namespace: nadNS},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: cfg},
				}
				nadB := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: nadBName, Namespace: nadNS},
					Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: cfg},
				}
				v, c, vmRef := setup("192.168.1.5", true,
					nadA, nadB, makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24"),
				)
				vm := v.Source.Inventory.(*mockInventory).vm
				vm.NICs = append(vm.NICs, vsphere.NIC{Network: vsphere.Ref{ID: srcNetID}, DeviceKey: 4002})
				vm.GuestNetworks = append(vm.GuestNetworks, vsphere.GuestNetwork{IP: "192.168.1.5", DeviceConfigId: 4002})
				v.Source.Inventory.(*mockInventory).vm = vm

				v.Plan.Referenced.Map.Network.Spec.Map = []v1beta1.NetworkPair{
					{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: srcNetID}}, Destination: v1beta1.DestinationNetwork{
						Type: planbase.Multus, Namespace: nadNS, Name: nadAName,
					}},
					{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: srcNetID}}, Destination: v1beta1.DestinationNetwork{
						Type: planbase.Multus, Namespace: nadNS, Name: nadBName,
					}},
				}

				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoIssue{
					Kind: planbase.CalicoIssueIPNotInSubnet, Network: netName, VLAN: 100, IP: "192.168.1.5",
				}))
			})
		})
	})

	Describe("Calico Primary network validation", func() {
		const (
			srcNetID = "src-1"
			netName  = "vlan100"
			targetNS = "test"
		)

		makeNetwork := func(spec map[string]interface{}) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.NetworkGVK)
			u.SetName(netName)
			if spec != nil {
				_ = unstructured.SetNestedField(u.Object, spec, "spec")
			}
			return u
		}
		makeIPPool := func(name, cidr string, allowedUses ...string) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.IPPoolGVK)
			u.SetName(name)
			_ = unstructured.SetNestedField(u.Object, cidr, "spec", "cidr")
			if len(allowedUses) > 0 {
				ifaces := make([]interface{}, len(allowedUses))
				for i, v := range allowedUses {
					ifaces[i] = v
				}
				_ = unstructured.SetNestedSlice(u.Object, ifaces, "spec", "allowedUses")
			}
			return u
		}
		makeUDNNamespace := func() *core.Namespace {
			return &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   targetNS,
					Labels: map[string]string{"k8s.ovn.org/primary-user-defined-network": ""},
				},
			}
		}
		l2Single := map[string]interface{}{
			"l2Bridge": map[string]interface{}{
				"vlans": []interface{}{
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(100)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
					},
				},
			},
		}
		l2Multi := map[string]interface{}{
			"l2Bridge": map[string]interface{}{
				"vlans": []interface{}{
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(100)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.100.0.0/24"}},
					},
					map[string]interface{}{
						"vlan":    map[string]interface{}{"id": int64(200)},
						"subnets": []interface{}{map[string]interface{}{"cidr": "10.200.0.0/24"}},
					},
				},
			},
		}

		// setupPrimary builds a Validator + fake client with a single
		// type: calico NetworkMap entry sourced from srcNetID. extraPairs
		// adds additional NetworkMap entries (for coexistence / misuse tests).
		setupPrimary := func(nicIP string, preserveIPs bool, dest v1beta1.DestinationNetwork, extraPairs []v1beta1.NetworkPair, registerCalicoKinds bool, k8sObjs ...runtime.Object) (*Validator, client.Client, ref.Ref) {
			scheme := runtime.NewScheme()
			_ = k8snet.AddToScheme(scheme)
			_ = core.AddToScheme(scheme)
			if registerCalicoKinds {
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(k8sObjs...).Build()

			vm := model.VM{
				VM1:  model.VM1{VM0: model.VM0{ID: "test-vm-id", Name: "test-vm"}},
				NICs: []vsphere.NIC{{Network: vsphere.Ref{ID: srcNetID}, DeviceKey: 4001}},
				GuestNetworks: []vsphere.GuestNetwork{
					{IP: nicIP, DeviceConfigId: 4001},
				},
			}
			inventory := &mockInventory{
				vm: vm,
				networks: map[string]model.Network{
					srcNetID: {Resource: model.Resource{ID: srcNetID}, Variant: vsphere.NetDvPortGroup, Key: srcNetID},
				},
			}
			plan := createPlan()
			plan.Spec.PreserveStaticIPs = preserveIPs
			pairs := []v1beta1.NetworkPair{
				{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: srcNetID}}, Destination: dest},
			}
			pairs = append(pairs, extraPairs...)
			plan.Referenced.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{Map: pairs},
			}
			ctx := plancontext.Context{Plan: plan, Source: plancontext.Source{Inventory: inventory}}
			return &Validator{Context: &ctx}, c, ref.Ref{Name: "test-vm-id", ID: "test-vm-id"}
		}

		kinds := func(issues []planbase.CalicoPrimaryIssue) []planbase.CalicoIssueKind {
			out := make([]planbase.CalicoIssueKind, 0, len(issues))
			for _, i := range issues {
				out = append(out, i.Kind)
			}
			return out
		}

		Describe("ValidateCalicoPrimary (plan-level)", func() {
			It("returns empty results when NetworkMap is nil", func() {
				v, c, _ := setupPrimary("10.100.0.5", false, v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}, nil, true)
				v.Plan.Referenced.Map.Network = nil
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache).NotTo(BeNil())
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("returns empty results when NetworkMap has no calico entries", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("happy path Case A (implicit L3 IPAM) — populates L3EligiblePools, no issues", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
				Expect(result.Cache.Primary.Network).To(BeEmpty())
				Expect(result.Cache.Primary.L3EligiblePools).To(HaveLen(1))
			})

			It("happy path (named single-VLAN network, explicit VLAN)", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
				Expect(result.Cache.Primary.Network).To(Equal(netName))
				Expect(result.Cache.Primary.VLAN.VID).To(Equal(uint16(100)))
				Expect(result.Cache.Primary.L2EligiblePools).To(HaveLen(1))
			})

			It("happy path Case C (named network, explicit VLAN)", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 200}}
				v, c, _ := setupPrimary("10.200.0.5", false, dest, nil, true,
					makeNetwork(l2Multi),
					makeIPPool("vlan200-pool", "10.200.0.0/24", "L2Workload"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary.VLAN.VID).To(Equal(uint16(200)))
			})

			It("emits PrimaryUnsupported when Calico CRDs are absent", func() {
				// The fake client does not natively return NoKindMatchError
				// for unregistered kinds, so we wrap it with an interceptor
				// that returns the error the real API server would on a
				// cluster without the projectcalico.org/v3 CRDs installed.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, _, _ := setupPrimary("10.244.0.5", false, dest, nil, true)
				ipoolGK := calicoclient.IPPoolGVK.GroupKind()
				ipoolListGVK := calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList")
				c := fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(ctx context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
							if list.GetObjectKind().GroupVersionKind() == ipoolListGVK {
								return &meta.NoKindMatchError{GroupKind: ipoolGK}
							}
							return nil
						},
					}).Build()
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryUnsupported))
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("emits PrimaryConflictsWithUDN when target namespace is UDN-labelled", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				udnNAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "udn-primary",
						Namespace: targetNS,
						Labels:    map[string]string{"k8s.ovn.org/user-defined-network": ""},
					},
				}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true, makeUDNNamespace(), udnNAD,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryConflictsWithUDN))
			})

			It("emits PrimaryFieldsMisplaced when the calico block is set on a non-pod entry", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Multus, Calico: &v1beta1.CalicoDestination{Network: "leaked"}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryFieldsMisplaced))
			})

			It("emits PrimaryFieldsMisplaced when calico.vlan is set without calico.network", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Vlan: 100}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ContainElement(planbase.CalicoIssuePrimaryFieldsMisplaced))
			})

			It("allows a plain pod entry to coexist with a calico-flagged entry", func() {
				// A calico-flagged entry IS a pod entry; a second plain pod
				// entry on another source network is not a plan-level
				// conflict (a VM with NICs on both would trip the existing
				// per-VM multiple-pod-mappings condition instead).
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				extra := []v1beta1.NetworkPair{
					{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}}, Destination: v1beta1.DestinationNetwork{Type: planbase.Pod}},
				}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, extra, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
			})

			It("emits PrimaryFieldsMisplaced for multiple calico-flagged entries", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				extra := []v1beta1.NetworkPair{
					{Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}}, Destination: v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}},
				}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, extra, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ContainElement(planbase.CalicoIssuePrimaryFieldsMisplaced))
			})

			It("emits PrimaryNetworkNotFound when calico.network names a missing CR", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: "missing", Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkNotFound))
			})

			It("emits PrimaryNetworkCRDAbsent when calico.network is set but Network CRD missing (IPPool present)", func() {
				// Calico installed (IPPool CRD present) but L2 feature not
				// shipped (Network CRD absent). User asked for L2 attach;
				// can't honour. Case A (no CalicoNetwork) would pass.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, _, _ := setupPrimary("10.100.0.5", false, dest, nil, true)
				networkGK := calicoclient.NetworkGVK.GroupKind()
				// Stand up a client where IPPool listing succeeds but
				// GetNetwork returns NoKindMatchError.
				scheme := runtime.NewScheme()
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
				c := fake.NewClientBuilder().WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							if obj.GetObjectKind().GroupVersionKind() == calicoclient.NetworkGVK {
								return &meta.NoKindMatchError{GroupKind: networkGK}
							}
							return nil
						},
					}).Build()
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkCRDAbsent))
			})

			It("emits PrimaryNetworkHasNoL2Bridge when the Network has no l2Bridge spec", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(map[string]interface{}{}),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkHasNoL2Bridge))
			})

			It("emits PrimaryNetworkHasNoVLANs when l2Bridge.vlans is empty", func() {
				emptyVLANs := map[string]interface{}{
					"l2Bridge": map[string]interface{}{"vlans": []interface{}{}},
				}
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true, makeNetwork(emptyVLANs))
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkHasNoVLANs))
			})

			It("emits PrimaryVLANRequired when calico.network is set without calico.vlan", func() {
				// A Network reference requires an explicit VLAN. The rejection
				// fires before the Network is fetched, so the Network need not
				// even exist.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryVLANRequired))
			})

			It("emits PrimaryVLANNotInNetwork when calico.vlan is absent from the Network's VLAN list", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 999}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true, makeNetwork(l2Single))
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryVLANNotInNetwork))
			})

			It("emits PrimaryNoEligibleIPPool when no L2Workload pool covers the VLAN subnet", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("workload-only", "10.100.0.0/24", "Workload"), // missing L2Workload
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNoEligibleIPPool))
			})

			It("emits PrimaryStaticIPsNotPreserved as a Warning when preserveStaticIPs is false", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Warnings)).To(ConsistOf(planbase.CalicoIssuePrimaryStaticIPsNotPreserved))
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
			})

			It("does not emit PrimaryStaticIPsNotPreserved when preserveStaticIPs is true", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, _ := setupPrimary("10.244.0.5", true, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Warnings).To(BeEmpty())
			})
		})

		Describe("CalicoPrimaryIssues (per-VM)", func() {
			It("returns nil when preserveStaticIPs is false", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, vmRef := setupPrimary("10.244.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("returns nil when cache is nil", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, _, vmRef := setupPrimary("10.244.0.5", true, dest, nil, true)
				issues, err := v.CalicoPrimaryIssues(vmRef, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("returns nil when cache.Primary is nil (plan-level failed or no calico entry)", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod}
				v, c, vmRef := setupPrimary("10.244.0.5", true, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
			})

			It("Case A: emits NoEligibleIPPool when no L3 pool covers the source IP", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, vmRef := setupPrimary("192.168.1.5", true, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoPrimaryIssue{
					VMRef: vmRef, Kind: planbase.CalicoIssuePrimaryNoEligibleIPPool, IP: "192.168.1.5",
				}))
			})

			It("Case B/C: emits IPNotInSubnet when source IP is outside the VLAN subnet", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, vmRef := setupPrimary("192.168.1.5", true, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoPrimaryIssue{
					VMRef: vmRef, Kind: planbase.CalicoIssuePrimaryIPNotInSubnet,
					Network: netName, VLAN: 100, IP: "192.168.1.5",
				}))
			})

			It("Case B/C: emits NoEligibleIPPool when no L2Workload pool covers the source IP", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, vmRef := setupPrimary("10.100.0.5", true, dest, nil, true,
					makeNetwork(l2Single),
					// pool covers VLAN subnet but excludes 10.100.0.5
					makeIPPool("vlan100-upper", "10.100.0.128/25", "L2Workload"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoPrimaryIssue{
					VMRef: vmRef, Kind: planbase.CalicoIssuePrimaryNoEligibleIPPool,
					Network: netName, VLAN: 100, IP: "10.100.0.5",
				}))
			})

			It("deduplicates identical per-NIC IP issues", func() {
				// Two NICs on the same source network, both mapped to the same
				// calico entry. Both NICs carry the same out-of-subnet IP so
				// both emit identical {Kind, Network, VLAN, IP}; dedup → one.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, vmRef := setupPrimary("192.168.1.5", true, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
				)
				vm := v.Source.Inventory.(*mockInventory).vm
				vm.NICs = append(vm.NICs, vsphere.NIC{Network: vsphere.Ref{ID: srcNetID}, DeviceKey: 4002})
				vm.GuestNetworks = append(vm.GuestNetworks, vsphere.GuestNetwork{IP: "192.168.1.5", DeviceConfigId: 4002})
				v.Source.Inventory.(*mockInventory).vm = vm

				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoPrimaryIssue{
					VMRef: vmRef, Kind: planbase.CalicoIssuePrimaryIPNotInSubnet,
					Network: netName, VLAN: 100, IP: "192.168.1.5",
				}))
			})

			It("emits PrimaryTooManyIPs when a calico-mapped NIC carries more than one IPv4", func() {
				// Calico's ipAddrs annotation accepts at most one IPv4 per
				// interface; a multi-IPv4 NIC would fail the pod at CNI ADD.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, vmRef := setupPrimary("10.244.0.5", true, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				vm := v.Source.Inventory.(*mockInventory).vm
				vm.GuestNetworks = append(vm.GuestNetworks, vsphere.GuestNetwork{IP: "10.244.0.6", DeviceConfigId: 4001})
				v.Source.Inventory.(*mockInventory).vm = vm

				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoPrimaryIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(HaveLen(1))
				Expect(issues[0].Kind).To(Equal(planbase.CalicoIssuePrimaryTooManyIPs))
			})
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
