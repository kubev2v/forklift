package template

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	builder "github.com/kubev2v/forklift/pkg/provider/builder"
	ec2base "github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/base"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
)

// Builder builds a KubeVirt VirtualMachineSpec by rendering a Go text/template
// with extracted VMBuildValues.
type Builder struct {
	base *ec2base.Base
}

// New creates a template Builder backed by the shared Base.
func New(b *ec2base.Base) *Builder {
	return &Builder{base: b}
}

// VirtualMachine builds a KubeVirt VirtualMachineSpec using a three-phase pipeline:
//  1. Extract -- reads the EC2 instance and resolves PVCs/network mappings into VMBuildValues
//  2. Render  -- executes a Go text/template (default or custom from ConfigMap) with the values
//  3. Unmarshal -- parses the rendered YAML into a cnv.VirtualMachineSpec
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, _ bool, _ bool) error {
	values, err := r.base.ExtractValues(vmRef, persistentVolumeClaims)
	if err != nil {
		return err
	}

	tmpl, err := r.loadVMTemplate()
	if err != nil {
		return err
	}

	spec, err := builder.RenderTemplate(tmpl, values)
	if err != nil {
		return err
	}

	*object = *spec
	return nil
}

// loadVMTemplate returns the Go text/template string for VM rendering.
func (r *Builder) loadVMTemplate() (string, error) {
	return DefaultVMTemplate, nil
}
