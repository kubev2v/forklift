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
	ocp "github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
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

// makeFelixConfiguration builds the cluster-wide "default" FelixConfiguration
// with spec.bpfEnabled set as given.
func makeFelixConfiguration(bpfEnabled bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(calicoclient.FelixConfigurationGVK)
	u.SetName("default")
	_ = unstructured.SetNestedField(u.Object, bpfEnabled, "spec", "bpfEnabled")
	return u
}

// makeNftablesFelixConfiguration builds the cluster-wide "default"
// FelixConfiguration with spec.nftablesMode set as given and bpfEnabled
// unset (BPF off). "Enabled" describes the only cluster flavour VRF
// networking supports.
func makeNftablesFelixConfiguration(mode string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(calicoclient.FelixConfigurationGVK)
	u.SetName("default")
	_ = unstructured.SetNestedField(u.Object, mode, "spec", "nftablesMode")
	return u
}

// withRouteTableRanges sets spec.routeTableRanges on a FelixConfiguration.
func withRouteTableRanges(u *unstructured.Unstructured, ranges ...[2]int64) *unstructured.Unstructured {
	entries := make([]interface{}, len(ranges))
	for i, r := range ranges {
		entries[i] = map[string]interface{}{"min": r[0], "max": r[1]}
	}
	_ = unstructured.SetNestedSlice(u.Object, entries, "spec", "routeTableRanges")
	return u
}

// withDefaultFelix appends a BPF-enabled "default" FelixConfiguration unless
// objs already carries a FelixConfiguration. l2Bridge networks are only valid
// on a BPF dataplane, so the Calico setup helpers install one by default;
// dataplane specs supply their own (bpfEnabled false, or per-node-only) to
// exercise the failure modes.
func withDefaultFelix(objs []runtime.Object) []runtime.Object {
	for _, o := range objs {
		if u, ok := o.(*unstructured.Unstructured); ok && u.GroupVersionKind() == calicoclient.FelixConfigurationGVK {
			return objs
		}
	}
	return append(objs, makeFelixConfiguration(true))
}

// Mock inventory struct and methods for testing
type mockInventory struct {
	ds       model.Datastore
	vm       model.VM
	networks map[string]model.Network // keyed by ID
	destVMs  []ocp.VM                 // served by List for destination inventories
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
	if l, ok := list.(*[]ocp.VM); ok {
		*l = m.destVMs
	}
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
		makeDisabledIPPool := func(name, cidr string, allowedUses ...string) *unstructured.Unstructured {
			u := makeIPPool(name, cidr, allowedUses...)
			_ = unstructured.SetNestedField(u.Object, true, "spec", "disabled")
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
		// VRF (routed, L3) Network helpers. vrfHostEntry builds one
		// spec.vrf.hostConfig entry; an empty selector omits the
		// nodeSelector field, so the entry applies to every node. The entry
		// names a host interface, keeping the VRFNoHostInterfaces check
		// quiet — vrfHostEntryNoInterfaces builds the offending shape.
		vrfHostEntry := func(selector string, routeTableIndex int64) map[string]interface{} {
			e := map[string]interface{}{
				"routeTableIndex": routeTableIndex,
				"hostInterfaces":  []interface{}{map[string]interface{}{"name": "eth1"}},
			}
			if selector != "" {
				e["nodeSelector"] = selector
			}
			return e
		}
		vrfHostEntryNoInterfaces := func(selector string, routeTableIndex int64) map[string]interface{} {
			e := vrfHostEntry(selector, routeTableIndex)
			delete(e, "hostInterfaces")
			return e
		}
		makeVRFNetworkNamed := func(name string, entries ...map[string]interface{}) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.NetworkGVK)
			u.SetName(name)
			list := make([]interface{}, len(entries))
			for i, e := range entries {
				list[i] = e
			}
			_ = unstructured.SetNestedField(u.Object, map[string]interface{}{
				"vrf": map[string]interface{}{"hostConfig": list},
			}, "spec")
			return u
		}
		makeVRFNetwork := func(entries ...map[string]interface{}) *unstructured.Unstructured {
			return makeVRFNetworkNamed(netName, entries...)
		}
		// makeBGPPeer builds a projectcalico.org/v3 BGPPeer whose
		// spec.network binds it to the named Network — the binding that
		// distributes a VRF network's routes across nodes. VRF fixtures
		// that must stay warning-free include one bound to their Network;
		// the VRFNoBGPPeer specs leave it out (or bind it elsewhere).
		makeBGPPeer := func(name, network string) *unstructured.Unstructured {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(calicoclient.BGPPeerGVK)
			u.SetName(name)
			_ = unstructured.SetNestedField(u.Object, network, "spec", "network")
			return u
		}

		// setup builds a Validator + fake client. nicIP is the NIC's guest IP,
		// preserveIPs controls Plan.Spec.PreserveStaticIPs. A BPF-enabled
		// "default" FelixConfiguration is installed unless the spec supplies
		// its own (see withDefaultFelix).
		setup := func(nicIP string, preserveIPs bool, k8sObjs ...runtime.Object) (*Validator, client.Client, ref.Ref) {
			scheme := runtime.NewScheme()
			_ = k8snet.AddToScheme(scheme)
			scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
			scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
			scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK.GroupVersion().WithKind("FelixConfigurationList"), &unstructured.UnstructuredList{})
			scheme.AddKnownTypeWithName(calicoclient.BGPPeerGVK, &unstructured.Unstructured{})
			scheme.AddKnownTypeWithName(calicoclient.BGPPeerGVK.GroupVersion().WithKind("BGPPeerList"), &unstructured.UnstructuredList{})
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(withDefaultFelix(k8sObjs)...).Build()

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
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
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
				// A Network reference requires an explicit VLAN. The Network
				// is fetched and classified first (an l2Bridge network here),
				// then the missing vlan is rejected before any VLAN matching.
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(0), makeNetwork(l2Multi),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANRequired))
			})

			It("emits NetworkNotFound (not VLANRequired) when the Network is missing and vlan is unset", func() {
				// The Network lookup precedes the vlan-required check: with a
				// missing Network the accurate report is NetworkNotFound —
				// asking for a vlan first would send the user chasing the
				// wrong fix.
				v, c, _ := setup("10.100.0.5", true, makeCalicoNAD(0))
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueNetworkNotFound))
			})

			It("accepts a VRF network reference and caches the L3-eligible pools", func() {
				// A VRF network is routed L3 — no VLAN to resolve, and the
				// BPF-dataplane requirement is l2Bridge-only. The cluster runs
				// the nftables dataplane (bpfEnabled unset → off), which the
				// VRF dataplane check requires; if the l2Bridge BPF check ran
				// on this plan it would report DataplaneNotBPF, so the empty
				// issue list also proves that check never runs for a VRF-only
				// plan. The all-nodes hostConfig entry (with a host
				// interface), unique route table, bound BGPPeer, and the
				// preserve-IPs plan (no pool pin needed) keep every viability
				// check quiet: fully clean result.
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(0), makeVRFNetwork(vrfHostEntry("", 101)),
					makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
					makeNftablesFelixConfiguration("Enabled"),
					makeBGPPeer("vrf-peer", netName),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Warnings).To(BeEmpty())
				entry := result.Cache.NADs[k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}]
				Expect(entry).NotTo(BeNil())
				Expect(entry.IsVRF).To(BeTrue())
				Expect(entry.VLAN).To(Equal(calicoclient.VLANEntry{}))
				Expect(entry.EligiblePools).To(HaveLen(1))
			})

			It("warns VRFVlanIgnored when the NAD sets a vlan on a VRF network", func() {
				// VLANs apply only to l2Bridge networks — the stray vlan is
				// surfaced as a warning; the reference stays valid and cached.
				// The bound BGPPeer, interface-carrying all-nodes entry, and
				// preserve-IPs plan keep the other VRF warnings out.
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeVRFNetwork(vrfHostEntry("", 101)),
					makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
					makeNftablesFelixConfiguration("Enabled"),
					makeBGPPeer("vrf-peer", netName),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
					NAD:     k8stypes.NamespacedName{Namespace: nadNS, Name: nadName},
					Kind:    planbase.CalicoIssueVRFVlanIgnored,
					Network: netName,
					VLAN:    100,
				}))
				entry := result.Cache.NADs[k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}]
				Expect(entry).NotTo(BeNil())
				Expect(entry.IsVRF).To(BeTrue())
			})

			Describe("VRF pool pinning", func() {
				nadKey := k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}

				// makeCalicoNADPinned is makeCalicoNAD with the NAD's IPAM
				// config pinning one IPPool via ipv4_pools, and without a
				// vlan (VRF NADs carry none).
				makeCalicoNADPinned := func(pool string) *k8snet.NetworkAttachmentDefinition {
					cfg := fmt.Sprintf(`{"type":"calico","network":"%s","ipam":{"type":"calico-ipam","ipv4_pools":[%q]}}`, netName, pool)
					return &k8snet.NetworkAttachmentDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: nadName, Namespace: nadNS},
						Spec:       k8snet.NetworkAttachmentDefinitionSpec{Config: cfg},
					}
				}

				It("warns VRFPoolNotPinned when the plan assigns fresh IPs and the NAD pins no ipv4_pools", func() {
					// Calico's IPAM is VRF-unaware: with no ipv4_pools pin and
					// the plan assigning fresh addresses (preserveStaticIPs
					// false), the NIC's address comes from whichever pool IPAM
					// selects. The bound BGPPeer, interface-carrying all-nodes
					// entry, and nftables dataplane keep every other warning
					// out, isolating the pool warning.
					v, c, _ := setup("10.100.0.5", false,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFPoolNotPinned,
						Network: netName,
					}))
					// Warn-class: the NAD stays cached for per-VM checks.
					Expect(result.Cache.NADs).To(HaveLen(1))
				})

				It("does not warn when the plan preserves static IPs", func() {
					// Deliberate: with preserveStaticIPs the addresses are
					// explicit via ipAddrs and already validated against pools
					// per-VM — the flat-pool risk exists only for freshly
					// assigned addresses. Same fixture as above except the
					// plan preserves.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(BeEmpty())
				})

				It("does not warn when the NAD pins ipv4_pools", func() {
					// Fresh assignment, but the NAD pins the VRF's pool — the
					// convention the warning asks for.
					v, c, _ := setup("10.100.0.5", false,
						makeCalicoNADPinned("default-pool"),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(BeEmpty())
				})

				It("warns per NAD, not per Network", func() {
					// The pin lives in each NAD's IPAM config, so two unpinned
					// NADs referencing the same VRF Network warn once each —
					// unlike the per-Network viability checks, which dedupe.
					secondNAD := &k8snet.NetworkAttachmentDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "calico-nad-2", Namespace: nadNS},
						Spec: k8snet.NetworkAttachmentDefinitionSpec{
							Config: fmt.Sprintf(`{"type":"calico","network":"%s"}`, netName),
						},
					}
					v, c, _ := setup("10.100.0.5", false,
						makeCalicoNAD(0), secondNAD,
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					v.Plan.Referenced.Map.Network.Spec.Map = append(
						v.Plan.Referenced.Map.Network.Spec.Map,
						v1beta1.NetworkPair{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}},
							Destination: v1beta1.DestinationNetwork{
								Type: planbase.Multus, Namespace: nadNS, Name: "calico-nad-2",
							},
						},
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(
						planbase.CalicoNADIssue{
							NAD:     nadKey,
							Kind:    planbase.CalicoIssueVRFPoolNotPinned,
							Network: netName,
						},
						planbase.CalicoNADIssue{
							NAD:     k8stypes.NamespacedName{Namespace: nadNS, Name: "calico-nad-2"},
							Kind:    planbase.CalicoIssueVRFPoolNotPinned,
							Network: netName,
						},
					))
				})
			})

			Describe("VRF viability", func() {
				nadKey := k8stypes.NamespacedName{Namespace: nadNS, Name: nadName}

				It("emits VRFNodeScoped as Critical when every entry is node-scoped and the plan sets no placement", func() {
					// Both entries carry a nodeSelector (empty is the
					// canonical all-nodes form, so this is a node subset)
					// and the plan pins nothing: VMs may land on uncovered
					// nodes and fail at CNI ADD. The bound BGPPeer keeps
					// VRFNoBGPPeer out; the entries carry host interfaces.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(
							vrfHostEntry("rack == 'a'", 101),
							vrfHostEntry("rack == 'b'", 102),
						),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Warnings).To(BeEmpty())
					Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFNodeScoped,
						Network: netName,
					}))
				})

				It("downgrades to VRFPlacementUnverified when the plan constrains VM placement", func() {
					// Same node-scoped hostConfig, but the plan pins VMs via
					// targetNodeSelector: the user took control of placement,
					// which Forklift cannot verify against Calico selectors.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(
							vrfHostEntry("rack == 'a'", 101),
							vrfHostEntry("rack == 'b'", 102),
						),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					v.Plan.Spec.TargetNodeSelector = map[string]string{"rack": "a"}
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFPlacementUnverified,
						Network: netName,
					}))
				})

				It("runs the viability checks once per referenced Network", func() {
					// Two NADs referencing the same VRF Network: the checks
					// dedupe per Network name, so the scoped-only hostConfig
					// yields exactly one issue, attached to the first NAD.
					secondNAD := &k8snet.NetworkAttachmentDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: "calico-nad-2", Namespace: nadNS},
						Spec: k8snet.NetworkAttachmentDefinitionSpec{
							Config: fmt.Sprintf(`{"type":"calico","network":"%s"}`, netName),
						},
					}
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0), secondNAD,
						makeVRFNetwork(vrfHostEntry("rack == 'a'", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					v.Plan.Referenced.Map.Network.Spec.Map = append(
						v.Plan.Referenced.Map.Network.Spec.Map,
						v1beta1.NetworkPair{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}},
							Destination: v1beta1.DestinationNetwork{
								Type: planbase.Multus, Namespace: nadNS, Name: "calico-nad-2",
							},
						},
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Warnings).To(BeEmpty())
					Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVRFNodeScoped))
					Expect(result.Cache.NADs).To(HaveLen(2))
				})

				It("emits VRFRouteTableReserved when a hostConfig entry claims a kernel table", func() {
					// 254 is the kernel's main table; a VRF must never own it.
					// The bound BGPPeer keeps the warning list empty.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 254)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:        nadKey,
						Kind:       planbase.CalicoIssueVRFRouteTableReserved,
						Network:    netName,
						RouteTable: 254,
					}))
					Expect(result.Warnings).To(BeEmpty())
				})

				It("emits VRFRouteTableConflict when another VRF Network shares the index via an all-nodes entry", func() {
					// The referenced Network claims table 101 on every node;
					// any other Network claiming 101 anywhere provably
					// overlaps it. The other Network need not be referenced by
					// the plan — the collision scan covers every Network CR.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeVRFNetworkNamed("other-vrf", vrfHostEntry("rack == 'b'", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:           nadKey,
						Kind:          planbase.CalicoIssueVRFRouteTableConflict,
						Network:       netName,
						RouteTable:    101,
						ConflictsWith: "other-vrf",
					}))
					Expect(result.Warnings).To(BeEmpty())
				})

				It("warns VRFRouteTablePossibleConflict when both sharing entries are node-scoped", func() {
					// Both entries carrying table 101 are selector-scoped, so
					// whether they ever land on the same node depends on what
					// the selectors match — unprovable without evaluating
					// them. The all-nodes entry on table 102 keeps the
					// NodeScoped warning out of the picture.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(
							vrfHostEntry("rack == 'a'", 101),
							vrfHostEntry("", 102),
						),
						makeVRFNetworkNamed("other-vrf", vrfHostEntry("rack == 'b'", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:           nadKey,
						Kind:          planbase.CalicoIssueVRFRouteTablePossibleConflict,
						Network:       netName,
						RouteTable:    101,
						ConflictsWith: "other-vrf",
					}))
				})

				It("does not report entries of the same Network sharing an index", func() {
					// Two hostConfig entries of one Network on the same table
					// are legitimate — same VRF, same table.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(
							vrfHostEntry("", 101),
							vrfHostEntry("rack == 'a'", 101),
						),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(BeEmpty())
				})

				It("emits VRFRouteTableConflict when the index falls inside explicit FelixConfiguration routeTableRanges", func() {
					felix := withRouteTableRanges(makeNftablesFelixConfiguration("Enabled"), [2]int64{100, 200})
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 150)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						felix,
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					// ConflictsWith stays empty: the collision is with the
					// FelixConfiguration, not another Network.
					Expect(result.Issues).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:        nadKey,
						Kind:       planbase.CalicoIssueVRFRouteTableConflict,
						Network:    netName,
						RouteTable: 150,
					}))
					Expect(result.Warnings).To(BeEmpty())
				})

				It("skips the FelixConfiguration sub-check when routeTableRanges is absent", func() {
					// With the field absent, Felix falls back to
					// version-dependent defaults that the validator
					// deliberately does not guess — table 42 would sit inside
					// a typical default range, and no issue may be raised.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 42)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(BeEmpty())
				})

				It("warns VRFNoBGPPeer when no BGPPeer names the network", func() {
					// VRF networks ship with local routes only; without a
					// BGPPeer whose spec.network names this Network, no
					// cross-node routes exist. No BGPPeer at all here. The
					// preserve-IPs plan and interface-carrying all-nodes entry
					// keep the other warnings out.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFNoBGPPeer,
						Network: netName,
					}))
					// Warn-class: the NAD stays cached for per-VM checks.
					Expect(result.Cache.NADs).To(HaveLen(1))
				})

				It("warns VRFNoBGPPeer when the only BGPPeer names a different network", func() {
					// A peer exists, but it distributes routes for another
					// Network — this VRF still has no cross-node routes.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("other-peer", "other-vrf"),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFNoBGPPeer,
						Network: netName,
					}))
				})

				It("still warns VRFNoBGPPeer when the BGPPeer kind is unknown to the API server", func() {
					// Older Calico installs don't ship the BGPPeer CRD (or its
					// spec.network field). Absence of evidence is not a bound
					// peer: with no peer able to name the network, the warning
					// must still fire. The scheme deliberately omits the
					// BGPPeer kinds; the interceptor turns the BGPPeer List
					// into the NoKindMatchError a real API server would return
					// and passes every other call through.
					v, _, _ := setup("10.100.0.5", true)
					scheme := runtime.NewScheme()
					_ = k8snet.AddToScheme(scheme)
					scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
					scheme.AddKnownTypeWithName(calicoclient.NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
					scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
					scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
					scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK, &unstructured.Unstructured{})
					scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK.GroupVersion().WithKind("FelixConfigurationList"), &unstructured.UnstructuredList{})
					bgpPeerGK := calicoclient.BGPPeerGVK.GroupKind()
					c := fake.NewClientBuilder().WithScheme(scheme).
						WithRuntimeObjects(
							makeCalicoNAD(0),
							makeVRFNetwork(vrfHostEntry("", 101)),
							makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
							makeNftablesFelixConfiguration("Enabled"),
						).
						WithInterceptorFuncs(interceptor.Funcs{
							List: func(ctx context.Context, inner client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
								if list.GetObjectKind().GroupVersionKind() == calicoclient.BGPPeerGVK.GroupVersion().WithKind("BGPPeerList") {
									return &meta.NoKindMatchError{GroupKind: bgpPeerGK}
								}
								return inner.List(ctx, list, opts...)
							},
						}).Build()
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFNoBGPPeer,
						Network: netName,
					}))
				})

				It("warns VRFNoHostInterfaces when a hostConfig entry names no host interfaces", func() {
					// The scoped entry names no hostInterfaces: VMs on its
					// nodes have no path off the node inside the VRF, so one
					// such entry warns for the whole Network. The all-nodes
					// entry (with an interface) keeps NodeScoped quiet, the
					// shared table within one Network is legitimate, and the
					// bound BGPPeer keeps VRFNoBGPPeer out.
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(0),
						makeVRFNetwork(
							vrfHostEntry("", 101),
							vrfHostEntryNoInterfaces("rack == 'a'", 101),
						),
						makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
						makeNftablesFelixConfiguration("Enabled"),
						makeBGPPeer("vrf-peer", netName),
					)
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Issues).To(BeEmpty())
					Expect(result.Warnings).To(ConsistOf(planbase.CalicoNADIssue{
						NAD:     nadKey,
						Kind:    planbase.CalicoIssueVRFNoHostInterfaces,
						Network: netName,
					}))
					// Warn-class: the NAD stays cached for per-VM checks.
					Expect(result.Cache.NADs).To(HaveLen(1))
				})

				DescribeTable("emits VRFDataplaneNotNftables when the dataplane is not nftables",
					func(felix *unstructured.Unstructured) {
						objs := []runtime.Object{
							makeCalicoNAD(0),
							makeVRFNetwork(vrfHostEntry("", 101)),
							makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
							makeBGPPeer("vrf-peer", netName),
						}
						if felix == nil {
							// Only a per-node FelixConfiguration, so the
							// "default" one is missing (and the setup helper's
							// auto-injection is suppressed): Felix runs on
							// built-in defaults, which are not nftables.
							perNode := makeFelixConfiguration(true)
							perNode.SetName("node.worker-1")
							felix = perNode
						}
						v, c, _ := setup("10.100.0.5", true, append(objs, felix)...)
						result, err := v.ValidateCalicoNADs(c)
						Expect(err).NotTo(HaveOccurred())
						Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVRFDataplaneNotNftables))
						// The finding is cluster-scoped; the NAD itself is
						// valid and stays cached so per-VM checks still run.
						Expect(result.Cache.NADs).To(HaveLen(1))
					},
					Entry("bpfEnabled true", makeFelixConfiguration(true)),
					Entry("nftablesMode absent (Felix default: Disabled)", makeFelixConfiguration(false)),
					Entry("nftablesMode Disabled", makeNftablesFelixConfiguration("Disabled")),
					// "Auto" leaves the dataplane to Felix's host detection,
					// which cannot be verified from here — treated as failing.
					Entry("nftablesMode Auto (indeterminate)", makeNftablesFelixConfiguration("Auto")),
					Entry("default FelixConfiguration missing", nil),
				)
			})

			Describe("mixed l2Bridge + VRF plans", func() {
				// An l2Bridge network requires the BPF dataplane; a VRF
				// network requires nftables. No FelixConfiguration satisfies
				// both, so the two network types are mutually exclusive per
				// cluster — a plan referencing both kinds of NAD always draws
				// at least one dataplane Critical, naming whichever side the
				// cluster cannot run.
				const vrfNetName = "vrf-net"
				const vrfNADName = "vrf-nad"

				mixedSetup := func(felix *unstructured.Unstructured) (*Validator, client.Client) {
					vrfNAD := &k8snet.NetworkAttachmentDefinition{
						ObjectMeta: metav1.ObjectMeta{Name: vrfNADName, Namespace: nadNS},
						Spec: k8snet.NetworkAttachmentDefinitionSpec{
							Config: fmt.Sprintf(`{"type":"calico","network":"%s"}`, vrfNetName),
						},
					}
					v, c, _ := setup("10.100.0.5", true,
						makeCalicoNAD(100), makeNetwork(l2Single),
						makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
						vrfNAD, makeVRFNetworkNamed(vrfNetName, vrfHostEntry("", 101)),
						makeIPPool("default-pool", "10.200.0.0/24", "Workload"),
						felix,
						makeBGPPeer("vrf-peer", vrfNetName),
					)
					v.Plan.Referenced.Map.Network.Spec.Map = append(
						v.Plan.Referenced.Map.Network.Spec.Map,
						v1beta1.NetworkPair{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}},
							Destination: v1beta1.DestinationNetwork{
								Type: planbase.Multus, Namespace: nadNS, Name: vrfNADName,
							},
						},
					)
					return v, c
				}

				It("reports only VRFDataplaneNotNftables on a BPF cluster", func() {
					// BPF satisfies the l2Bridge side and fails the VRF side.
					v, c := mixedSetup(makeFelixConfiguration(true))
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVRFDataplaneNotNftables))
					Expect(result.Cache.NADs).To(HaveLen(2))
				})

				It("reports only DataplaneNotBPF on an nftables cluster", func() {
					// nftables satisfies the VRF side and fails the l2Bridge
					// side.
					v, c := mixedSetup(makeNftablesFelixConfiguration("Enabled"))
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueDataplaneNotBPF))
					Expect(result.Cache.NADs).To(HaveLen(2))
				})

				It("reports both dataplane Criticals on a cluster running neither BPF nor nftables", func() {
					// An iptables cluster (bpfEnabled false, nftablesMode
					// unset) can honour neither network type; both sides
					// report, each naming its own remedy.
					v, c := mixedSetup(makeFelixConfiguration(false))
					result, err := v.ValidateCalicoNADs(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(nadIssueKinds(result.Issues)).To(ConsistOf(
						planbase.CalicoIssueDataplaneNotBPF,
						planbase.CalicoIssueVRFDataplaneNotNftables,
					))
					Expect(result.Cache.NADs).To(HaveLen(2))
				})
			})

			It("does not emit a dataplane issue when FelixConfiguration has bpfEnabled true", func() {
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					makeFelixConfiguration(true),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.NADs).To(HaveLen(1))
			})

			It("emits DataplaneNotBPF when FelixConfiguration has bpfEnabled false", func() {
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					makeFelixConfiguration(false),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueDataplaneNotBPF))
				// The issue is plan-scoped; the NAD's own configuration is
				// valid and stays cached so per-VM checks still run.
				Expect(result.Cache.NADs).To(HaveLen(1))
			})

			It("emits DataplaneNotBPF when the default FelixConfiguration is missing", func() {
				// Only a per-node FelixConfiguration exists (suppresses the
				// setup helper's auto-injected BPF-enabled default). Felix
				// then runs the cluster default dataplane, which is not BPF.
				perNode := makeFelixConfiguration(true)
				perNode.SetName("node.worker-1")
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					perNode,
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueDataplaneNotBPF))
			})

			It("emits DataplaneNotBPF once for a plan with multiple l2Bridge NADs", func() {
				// The dataplane is a cluster property; two healthy l2Bridge
				// NADs must not double-report it.
				secondNAD := &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{Name: "calico-nad-2", Namespace: nadNS},
					Spec: k8snet.NetworkAttachmentDefinitionSpec{
						Config: fmt.Sprintf(`{"type":"calico","network":"%s","vlan":100}`, netName),
					},
				}
				v, c, _ := setup("10.100.0.5", true,
					makeCalicoNAD(100), secondNAD, makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					makeFelixConfiguration(false),
				)
				v.Plan.Referenced.Map.Network.Spec.Map = append(
					v.Plan.Referenced.Map.Network.Spec.Map,
					v1beta1.NetworkPair{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "src-2"}},
						Destination: v1beta1.DestinationNetwork{
							Type: planbase.Multus, Namespace: nadNS, Name: "calico-nad-2",
						},
					},
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueDataplaneNotBPF))
				Expect(result.Cache.NADs).To(HaveLen(2))
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
					makeIPPool("cluster-default", "10.0.0.0/8", "L2Workload"), // pool too large
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANHasNoIPPool))
			})

			It("emits VLANHasNoIPPool when the only covering pool is disabled", func() {
				// The pool's CIDR matches the VLAN subnet, but a disabled pool
				// can never satisfy a CNI ADD — it must not pass validation.
				v, c, _ := setup("10.100.0.5", false,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeDisabledIPPool("vlan100-disabled", "10.100.0.0/24", "L2Workload"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(nadIssueKinds(result.Issues)).To(ConsistOf(planbase.CalicoIssueVLANHasNoIPPool))
			})

			It("emits VLANHasNoIPPool when the only covering pool lacks the L2Workload use", func() {
				// A Workload-only pool covers the subnet, but Calico won't
				// assign L2-attached addresses from it — it must not pass
				// validation.
				v, c, _ := setup("10.100.0.5", false,
					makeCalicoNAD(100), makeNetwork(l2Single),
					makeIPPool("vlan100-workload-only", "10.100.0.0/24", "Workload"),
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
				// and a second L3-mode NAD (Warn NADMissingNetwork). Both slices
				// populate independently: items disjoint, no cross-contamination
				// between Critical and Warn.
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
					makeIPPool("vlan200-pool", "10.200.0.0/24", "L2Workload"),
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
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
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
					makeIPPool("vlan100-upper", "10.100.0.128/25", "L2Workload"), // covers VLAN but not 10.100.0.5
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
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
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
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
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

			It("emits IPNotInIPPool for a VRF-backed NAD when no eligible pool covers the IP", func() {
				// A VRF network has no subnets, so the subnet check is skipped
				// — the preserved IP is validated directly against the
				// L3-eligible pools. Here the only pool misses the IP.
				v, c, vmRef := setup("10.100.0.5", true,
					makeCalicoNAD(0), makeVRFNetwork(vrfHostEntry("", 101)),
					makeIPPool("other-pool", "10.200.0.0/24", "Workload"),
					makeNftablesFelixConfiguration("Enabled"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(ConsistOf(planbase.CalicoIssue{
					Kind: planbase.CalicoIssueIPNotInIPPool, Network: netName, IP: "10.100.0.5",
				}))
			})

			It("emits TooManyIPs when a VRF-backed NIC carries more than one IPv4", func() {
				// Calico's ipAddrs annotation still caps at one IPv4 per
				// interface on a VRF network.
				v, c, vmRef := setup("10.100.0.5", true,
					makeCalicoNAD(0), makeVRFNetwork(vrfHostEntry("", 101)),
					makeIPPool("default-pool", "10.100.0.0/24", "Workload"),
					makeNftablesFelixConfiguration("Enabled"),
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

			It("returns no issues for a VRF-backed NAD when preserveStaticIPs is false", func() {
				// The IP is outside every pool, but preservation is off.
				v, c, vmRef := setup("10.100.0.5", false,
					makeCalicoNAD(0), makeVRFNetwork(vrfHostEntry("", 101)),
					makeIPPool("other-pool", "10.200.0.0/24", "Workload"),
					makeNftablesFelixConfiguration("Enabled"),
				)
				result, err := v.ValidateCalicoNADs(c)
				Expect(err).NotTo(HaveOccurred())
				issues, err := v.CalicoVMIssues(vmRef, result.Cache)
				Expect(err).NotTo(HaveOccurred())
				Expect(issues).To(BeEmpty())
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
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
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
		// VRF (routed, L3) Network spec — a real network type, but not one
		// the calico-primary path supports.
		vrfSpec := map[string]interface{}{
			"vrf": map[string]interface{}{
				"hostConfig": []interface{}{
					map[string]interface{}{"routeTableIndex": int64(101)},
				},
			},
		}

		// setupPrimary builds a Validator + fake client with a single
		// type: calico NetworkMap entry sourced from srcNetID. extraPairs
		// adds additional NetworkMap entries (for coexistence / misuse tests).
		// With registerCalicoKinds, a BPF-enabled "default" FelixConfiguration
		// is installed unless the spec supplies its own (see withDefaultFelix).
		setupPrimary := func(nicIP string, preserveIPs bool, dest v1beta1.DestinationNetwork, extraPairs []v1beta1.NetworkPair, registerCalicoKinds bool, k8sObjs ...runtime.Object) (*Validator, client.Client, ref.Ref) {
			scheme := runtime.NewScheme()
			_ = k8snet.AddToScheme(scheme)
			_ = core.AddToScheme(scheme)
			if registerCalicoKinds {
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.NetworkGVK.GroupVersion().WithKind("NetworkList"), &unstructured.UnstructuredList{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.IPPoolGVK.GroupVersion().WithKind("IPPoolList"), &unstructured.UnstructuredList{})
				scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK, &unstructured.Unstructured{})
				scheme.AddKnownTypeWithName(calicoclient.FelixConfigurationGVK.GroupVersion().WithKind("FelixConfigurationList"), &unstructured.UnstructuredList{})
				k8sObjs = withDefaultFelix(k8sObjs)
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

			It("happy path non-L2 (implicit L3 IPAM) — populates L3EligiblePools, no issues", func() {
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

			It("happy path L2-attach (single-VLAN Network, explicit VLAN)", func() {
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

			It("happy path L2-attach (multi-VLAN Network, explicit VLAN)", func() {
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

			It("emits PrimaryFieldsMisplaced when the calico block is set on a multus entry", func() {
				dest := v1beta1.DestinationNetwork{
					Type: planbase.Multus, Namespace: "ns", Name: "nad",
					Calico: &v1beta1.CalicoDestination{},
				}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryFieldsMisplaced))
				// A multus block never seeds the primary cache.
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("emits PrimaryFieldsMisplaced when the calico block is set on an ignored entry", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Ignored, Calico: &v1beta1.CalicoDestination{}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryFieldsMisplaced))
			})

			It("emits PrimaryNetworkNotFound when calico.network names a missing CR", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: "missing", Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkNotFound))
			})

			It("emits PrimaryNetworkCRDAbsent when calico.network is set but Network CRD missing (IPPool present)", func() {
				// Calico installed (IPPool CRD present) but the install does
				// not ship the Network CRD. User asked for L2 attach; can't
				// honour. The non-L2 case (no calico.network) would pass.
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
				// A Network reference requires an explicit VLAN. The Network
				// is fetched and classified first (an l2Bridge network here),
				// then the missing vlan is rejected before any VLAN matching.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryVLANRequired))
			})

			It("emits PrimaryNetworkNotFound (not PrimaryVLANRequired) when the Network is missing and calico.vlan is unset", func() {
				// The Network lookup precedes the vlan-required check: with a
				// missing Network the accurate report is NetworkNotFound —
				// asking for a vlan first would send the user chasing the
				// wrong fix.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: "missing"}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkNotFound))
			})

			It("emits PrimaryNetworkTypeUnsupported when calico.network names a VRF network", func() {
				// A VRF reference legitimately carries no vlan; the misleading
				// PrimaryVLANRequired must not fire, and the primary cache
				// must not seed.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(vrfSpec),
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkTypeUnsupported))
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("emits PrimaryNetworkTypeUnsupported even when calico.vlan is set on a VRF network", func() {
				// Same root cause — the network type is wrong; a VLAN check
				// against a VRF network would mislead.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(vrfSpec),
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryNetworkTypeUnsupported))
				Expect(result.Cache.Primary).To(BeNil())
			})

			It("does not emit a dataplane issue when FelixConfiguration has bpfEnabled true", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					makeFelixConfiguration(true),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
			})

			It("emits PrimaryDataplaneNotBPF when FelixConfiguration has bpfEnabled false", func() {
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					makeFelixConfiguration(false),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryDataplaneNotBPF))
				// The issue is plan-scoped; the mapping itself is valid and
				// the cache still seeds so per-VM checks run.
				Expect(result.Cache.Primary).NotTo(BeNil())
			})

			It("emits PrimaryDataplaneNotBPF when the default FelixConfiguration is missing", func() {
				// Only a per-node FelixConfiguration exists (suppresses the
				// setup helper's auto-injected BPF-enabled default). Felix
				// then runs the cluster default dataplane, which is not BPF.
				perNode := makeFelixConfiguration(true)
				perNode.SetName("node.worker-1")
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{Network: netName, Vlan: 100}}
				v, c, _ := setupPrimary("10.100.0.5", false, dest, nil, true,
					makeNetwork(l2Single),
					makeIPPool("vlan100-pool", "10.100.0.0/24", "L2Workload"),
					perNode,
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(kinds(result.Issues)).To(ConsistOf(planbase.CalicoIssuePrimaryDataplaneNotBPF))
			})

			It("does not run the dataplane check for the non-L2 case (no calico.network)", func() {
				// The non-L2 case is plain L3 IPAM — no l2Bridge network is engaged, so
				// a non-BPF dataplane is fine.
				dest := v1beta1.DestinationNetwork{Type: planbase.Pod, Calico: &v1beta1.CalicoDestination{}}
				v, c, _ := setupPrimary("10.244.0.5", false, dest, nil, true,
					makeIPPool("default-ipv4-ippool", "10.244.0.0/16"),
					makeFelixConfiguration(false),
				)
				result, err := v.ValidateCalicoPrimary(c)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Issues).To(BeEmpty())
				Expect(result.Cache.Primary).NotTo(BeNil())
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

			It("non-L2: emits NoEligibleIPPool when no L3 pool covers the source IP", func() {
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

			It("L2-attach: emits IPNotInSubnet when source IP is outside the VLAN subnet", func() {
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

			It("L2-attach: emits NoEligibleIPPool when no L2Workload pool covers the source IP", func() {
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
