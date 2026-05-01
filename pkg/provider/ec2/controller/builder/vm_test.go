package builder

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
)

func TestBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EC2 controller builder")
}

var _ = Describe("EC2 Builder VirtualMachine", func() {
	var (
		builder  *Builder
		fakeInv  *FakeInventory
		vmRef    ref.Ref
		vmSpec   *cnv.VirtualMachineSpec
		instance *model.InstanceDetails
	)

	BeforeEach(func() {
		fakeInv = NewFakeInventory()

		instance = &model.InstanceDetails{
			ID:   "i-123",
			Name: "test-vm",
		}
		instance.InstanceId = aws.String("i-123")
		instance.InstanceType = ec2types.InstanceTypeM5Large

		fakeInv.VMs["i-123"] = &web.VM{
			Resource: web.Resource{ID: "i-123", Name: "test-vm"},
			Object:   instance,
		}

		vmRef = ref.Ref{ID: "i-123"}

		networkMap := &api.NetworkMap{
			Spec: api.NetworkMapSpec{
				Map: []api.NetworkPair{},
			},
		}

		ctx := testutil.NewContextBuilder().
			WithNetworkMap(networkMap).
			Build()
		ctx.Source.Inventory = fakeInv

		builder = New(ctx)
	})

	Describe("RunStrategy preservation", func() {
		table.DescribeTable("should not override RunStrategy set by the plan",
			func(presetStrategy cnv.VirtualMachineRunStrategy) {
				vmSpec = &cnv.VirtualMachineSpec{
					RunStrategy: &presetStrategy,
				}

				err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{}, false, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(vmSpec.RunStrategy).NotTo(BeNil())
				Expect(*vmSpec.RunStrategy).To(Equal(presetStrategy),
					"Builder must not override RunStrategy; the plan-level determineRunStrategy controls it")
			},
			table.Entry("preserves RunStrategyAlways", cnv.RunStrategyAlways),
			table.Entry("preserves RunStrategyHalted", cnv.RunStrategyHalted),
			table.Entry("preserves RunStrategyManual", cnv.RunStrategyManual),
			table.Entry("preserves RunStrategyRerunOnFailure", cnv.RunStrategyRerunOnFailure),
		)

		It("should not set RunStrategy when none was pre-set", func() {
			vmSpec = &cnv.VirtualMachineSpec{}

			err := builder.VirtualMachine(vmRef, vmSpec, []*core.PersistentVolumeClaim{}, false, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(vmSpec.RunStrategy).To(BeNil(),
				"Builder must not set RunStrategy; the plan-level determineRunStrategy controls it")
		})
	})
})

// FakeInventory implements the base.Client interface for testing.
type FakeInventory struct {
	VMs   map[string]*web.VM
	Error error
}

var _ base.Client = (*FakeInventory)(nil)

func NewFakeInventory() *FakeInventory {
	return &FakeInventory{
		VMs: make(map[string]*web.VM),
	}
}

func (f *FakeInventory) Finder() base.Finder { return nil }

func (f *FakeInventory) Get(resource interface{}, id string) error {
	return errors.New("not implemented")
}

func (f *FakeInventory) List(list interface{}, param ...base.Param) error {
	return errors.New("not implemented")
}

func (f *FakeInventory) Watch(resource interface{}, h base.EventHandler) (*base.Watch, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeInventory) Find(resource interface{}, r ref.Ref) error {
	if f.Error != nil {
		return f.Error
	}

	switch res := resource.(type) {
	case *web.VM:
		id := r.ID
		if id == "" {
			id = r.Name
		}
		if vm, ok := f.VMs[id]; ok {
			*res = *vm
			return nil
		}
		return errors.New("VM not found")
	}
	return errors.New("unknown resource type")
}

func (f *FakeInventory) VM(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeInventory) Workload(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeInventory) Network(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeInventory) Storage(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *FakeInventory) Host(r *ref.Ref) (interface{}, error) {
	return nil, errors.New("not implemented")
}
