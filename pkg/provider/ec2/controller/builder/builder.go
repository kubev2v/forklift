package builder

import (
	"os"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ec2base "github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/imperative"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/template"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
)

// VMBuilder is the interface for the VirtualMachine build strategy.
type VMBuilder interface {
	VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec,
		pvcs []*core.PersistentVolumeClaim, usesInstanceType bool,
		sortVolumesByLibvirt bool) error
}

// Re-export base types so external consumers do not need to import base/.
type VolumeInfo = ec2base.VolumeInfo

// Builder dispatches VirtualMachine calls to either the imperative or template
// sub-builder, selected once at construction time via the EC2_BUILD_MODE env var.
// All other base.Builder interface methods are promoted from the embedded *Base.
type Builder struct {
	*ec2base.Base
	vmBuilder VMBuilder
}

// New creates a new EC2 Builder with plan context.
// EC2_BUILD_MODE=template selects the template pipeline; anything else (including
// unset) selects the imperative map* builder.
func New(ctx *plancontext.Context) *Builder {
	b := ec2base.New(ctx)
	var vm VMBuilder
	if os.Getenv("EC2_BUILD_MODE") == "template" {
		vm = template.New(b)
	} else {
		vm = imperative.New(b)
	}
	return &Builder{Base: b, vmBuilder: vm}
}

// VirtualMachine delegates to the selected build strategy.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec,
	pvcs []*core.PersistentVolumeClaim, usesInstanceType bool,
	sortVolumesByLibvirt bool) error {
	return r.vmBuilder.VirtualMachine(vmRef, object, pvcs, usesInstanceType, sortVolumesByLibvirt)
}
