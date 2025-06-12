package ocp

import (
	"path"
	"strconv"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
)

// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

const (
	MaxDetail = base.MaxDetail
)

// Types
type Model = base.Model
type ListOptions = base.ListOptions
type Ref = base.Ref

// k8s Resource.
type Resource interface {
	meta.Object
	runtime.Object
}

// Base k8s model.
type Base struct {
	// Object UID.
	UID string `sql:"pk"`
	// Resource version.
	Version string `sql:"d0"`
	// Namespace.
	Namespace string `sql:"key"`
	// Name.
	Name string `sql:"key"`
	// Labels
	labels libmodel.Labels
}

// Populate fields with the specified k8s resource.
func (m *Base) With(r Resource) {
	m.UID = string(r.GetUID())
	m.Version = r.GetResourceVersion()
	m.Namespace = r.GetNamespace()
	m.Name = r.GetName()
}

// Get kubernetes resource version.
func (m *Base) ResourceVersion() uint64 {
	n, _ := strconv.ParseUint(m.Version, 10, 64)
	return n
}

func (m *Base) Pk() string {
	return m.UID
}

func (m *Base) String() string {
	return path.Join(m.Namespace, m.Name)
}

func (m *Base) Labels() libmodel.Labels {
	return m.labels
}

// Provider
type Provider struct {
	Base
	Type   string       `sql:""`
	Object api.Provider `sql:""`
}

func (m *Provider) With(p *api.Provider) {
	m.Base.With(p)
	m.Type = p.Type().String()
	m.Object = *p
}

// Namespace
type Namespace struct {
	Base
	Object core.Namespace `sql:""`
}

func (m *Namespace) With(n *core.Namespace) {
	m.Base.With(n)
	m.Object = *n
}

// StorageClass
type StorageClass struct {
	Base
	Object storage.StorageClass `sql:""`
}

func (m *StorageClass) With(s *storage.StorageClass) {
	m.Base.With(s)
	m.Object = *s
}

// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	Base
	Object net.NetworkAttachmentDefinition `sql:""`
}

func (m *NetworkAttachmentDefinition) With(n *net.NetworkAttachmentDefinition) {
	m.Base.With(n)
	m.Object = *n
}

// InstanceTypes
type InstanceType struct {
	Base
	Object instancetype.VirtualMachineInstancetype `sql:""`
}

func (m *InstanceType) With(i *instancetype.VirtualMachineInstancetype) {
	m.Base.With(i)
	m.Object = *i
}

// ClusterInstanceTypes
type ClusterInstanceType struct {
	Base
	Object instancetype.VirtualMachineClusterInstancetype `sql:""`
}

func (m *ClusterInstanceType) With(i *instancetype.VirtualMachineClusterInstancetype) {
	m.Base.With(i)
	m.Object = *i
}

// VM
type VM struct {
	Base
	Object cnv.VirtualMachine `sql:""`
}

func (m *VM) With(v *cnv.VirtualMachine) {
	m.Base.With(v)
	m.Object = *v
}
