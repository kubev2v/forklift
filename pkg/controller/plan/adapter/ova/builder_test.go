package ova

import (
	"errors"
	"fmt"
	"testing"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/onsi/gomega"
	cnv "kubevirt.io/api/core/v1"
)

// Mock destination inventory for testing
type MockDestinationInventory struct{}

var errNotImplemented = errors.New("not implemented in mock")

func (m *MockDestinationInventory) Find(model interface{}, ref base.Ref) error { return nil }
func (m *MockDestinationInventory) Get(resource interface{}, id string) error  { return nil }
func (m *MockDestinationInventory) Watch(resource interface{}, h base.EventHandler) (*base.Watch, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) VM(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) Workload(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) Network(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) Storage(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) Host(ref *base.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (m *MockDestinationInventory) Finder() base.Finder { return nil }

func (m *MockDestinationInventory) List(list interface{}, param ...base.Param) error {
	vms := list.(*[]ocp.VM)
	*vms = []ocp.VM{
		{
			Resource: ocp.Resource{Name: "existing-vm-1", Namespace: "test-namespace"},
			Object: cnv.VirtualMachine{
				Spec: cnv.VirtualMachineSpec{
					Template: &cnv.VirtualMachineInstanceTemplateSpec{
						Spec: cnv.VirtualMachineInstanceSpec{
							Domain: cnv.DomainSpec{
								Devices: cnv.Devices{
									Interfaces: []cnv.Interface{{MacAddress: "00:11:22:33:44:55"}},
								},
							},
						},
					},
				},
			},
		},
		{
			Resource: ocp.Resource{Name: "existing-vm-2", Namespace: "test-namespace"},
			Object: cnv.VirtualMachine{
				Spec: cnv.VirtualMachineSpec{
					Template: &cnv.VirtualMachineInstanceTemplateSpec{
						Spec: cnv.VirtualMachineInstanceSpec{
							Domain: cnv.DomainSpec{
								Devices: cnv.Devices{
									Interfaces: []cnv.Interface{{MacAddress: ""}}, // Empty MAC
								},
							},
						},
					},
				},
			},
		},
		{
			Resource: ocp.Resource{Name: "existing-vm-3", Namespace: "test-namespace"},
			Object: cnv.VirtualMachine{
				Spec: cnv.VirtualMachineSpec{
					Template: &cnv.VirtualMachineInstanceTemplateSpec{
						Spec: cnv.VirtualMachineInstanceSpec{
							Domain: cnv.DomainSpec{
								Devices: cnv.Devices{
									Interfaces: []cnv.Interface{{MacAddress: "aa:bb:cc:dd:ee:ff"}},
								},
							},
						},
					},
				},
			},
		},
	}
	return nil
}

func createMockContext() *plancontext.Context {
	return &plancontext.Context{
		Destination: plancontext.Destination{Inventory: &MockDestinationInventory{}},
		Log:         logging.WithName("test"),
	}
}

func TestMacConflicts(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name              string
		sourceMACs        []string
		expectedConflicts []string
	}{
		{
			name:              "empty MAC - no conflict",
			sourceMACs:        []string{""},
			expectedConflicts: nil,
		},
		{
			name:              "conflicting MAC",
			sourceMACs:        []string{"00:11:22:33:44:55"},
			expectedConflicts: []string{"test-namespace/existing-vm-1"},
		},
		{
			name:              "non-conflicting MAC",
			sourceMACs:        []string{"99:88:77:66:55:44"},
			expectedConflicts: nil,
		},
		{
			name:              "mixed MACs - empty and conflicting",
			sourceMACs:        []string{"", "00:11:22:33:44:55", "99:88:77:66:55:44"},
			expectedConflicts: []string{"test-namespace/existing-vm-1"},
		},
		{
			name:              "multiple empty MACs",
			sourceMACs:        []string{"", ""},
			expectedConflicts: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			builder := &Builder{Context: createMockContext()}

			// Create source VM with specified MACs
			sourceVM := &model.VM{NICs: []ova.NIC{}}
			for i, mac := range testCase.sourceMACs {
				sourceVM.NICs = append(sourceVM.NICs, ova.NIC{
					Name: fmt.Sprintf("eth%d", i),
					MAC:  mac,
				})
			}

			conflicts, err := builder.macConflicts(sourceVM)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(conflicts).To(gomega.Equal(testCase.expectedConflicts))
		})
	}
}
