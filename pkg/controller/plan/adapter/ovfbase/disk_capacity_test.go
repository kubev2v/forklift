package ovfbase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ovfmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/onsi/gomega"
)

func TestDiskCapacityMB(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int64
		units       string
		expected    int64
		expectError bool
	}{
		{
			name:     "raw bytes (1nisim-rhel9-efi)",
			capacity: 10737418240,
			units:    "byte",
			expected: 10240,
		},
		{
			name:     "gibibytes (ameen-opensuse-btrfs)",
			capacity: 30,
			units:    "byte * 2^30",
			expected: 30720,
		},
		{
			name:     "byte * 2^20",
			capacity: 1,
			units:    "byte * 2^20",
			expected: 1,
		},
		{
			name:        "unsupported units",
			capacity:    10,
			units:       "kilobytes",
			expectError: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result, err := diskCapacityMB(testCase.capacity, testCase.units)
			if testCase.expectError {
				g.Expect(err).To(gomega.HaveOccurred(), fmt.Sprintf("expected an error for input: %v", testCase.units))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("did not expect an error for input: %v, but got: %v", testCase.units, err))
				g.Expect(result).To(gomega.Equal(testCase.expected), fmt.Sprintf("expected %v, but got %v", testCase.expected, result))
			}
		})
	}
}

func TestBuilderTasks_diskCapacityWithAllocationUnits(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vm := model.VM{
		VM1: model.VM1{
			VM0: model.VM0{
				ID:   "opensuse-vm",
				Name: "ameen-opensuse-btrfs",
			},
		},
		Disks: []ovfmodel.Disk{
			{
				Base: ovfmodel.Base{
					ID:   "disk-1",
					Name: "ameen-opensuse-btrfs-disk1.vmdk",
				},
				FilePath:                "/ova/msafra/ameen-opensuse-btrfs.ova",
				Capacity:                30,
				CapacityAllocationUnits: "byte * 2^30",
			},
		},
	}

	builder := &Builder{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Inventory: &tasksTestInventory{vm: vm},
			},
		},
	}

	tasks, err := builder.Tasks(ref.Ref{ID: vm.ID, Name: vm.Name})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(tasks).To(gomega.HaveLen(1))
	g.Expect(tasks[0].Progress.Total).To(gomega.Equal(int64(30720)))
}

type tasksTestInventory struct {
	vm model.VM
}

func (m *tasksTestInventory) Find(resource interface{}, _ ref.Ref) error {
	switch res := resource.(type) {
	case *model.VM:
		*res = m.vm
		return nil
	default:
		return errors.New("unexpected resource type")
	}
}

func (m *tasksTestInventory) Finder() web.Finder {
	return nil
}

func (m *tasksTestInventory) Get(_ interface{}, _ string) error {
	return errors.New("not implemented")
}

func (m *tasksTestInventory) List(_ interface{}, _ ...web.Param) error {
	return errors.New("not implemented")
}

func (m *tasksTestInventory) Watch(_ interface{}, _ base.EventHandler) (*base.Watch, error) {
	return nil, errors.New("not implemented")
}

func (m *tasksTestInventory) VM(_ *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *tasksTestInventory) Workload(_ *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *tasksTestInventory) Network(_ *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *tasksTestInventory) Storage(_ *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *tasksTestInventory) Host(_ *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}
