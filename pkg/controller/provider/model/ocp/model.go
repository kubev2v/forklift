package ocp

import (
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/client-go/api/v1"
	"path"
	"strconv"
)

// Errors
var NotFound = libmodel.NotFound

//
// Types
type Model = libmodel.Model

//
// k8s Resource.
type Resource interface {
	meta.Object
	runtime.Object
}

//
// Base k8s model.
type Base struct {
	// PK
	PK string `sql:"pk"`
	// Object UID.
	UID string `sql:"d0"`
	// Resource version.
	Version string `sql:"d0"`
	// Namespace.
	Namespace string `sql:"key"`
	// Name.
	Name string `sql:"key"`
	// Labels
	labels libmodel.Labels
}

//
// Populate fields with the specified k8s resource.
func (m *Base) With(r Resource) {
	m.UID = string(r.GetUID())
	m.Version = r.GetResourceVersion()
	m.Namespace = r.GetNamespace()
	m.Name = r.GetName()
}

//
// Get kubernetes resource version.
// Needed by the data reconciler.
func (m *Base) ResourceVersion() uint64 {
	n, _ := strconv.ParseUint(m.Version, 10, 64)
	return n
}

func (m *Base) Pk() string {
	return m.PK
}

func (m *Base) String() string {
	return path.Join(m.Namespace, m.Name)
}

func (m *Base) Equals(other Model) bool {
	if b, cast := other.(*Base); cast {
		return m.Namespace == b.Namespace &&
			m.Name == b.Name
	}

	return false
}

func (m *Base) Labels() libmodel.Labels {
	return m.labels
}

//
// Provider
type Provider struct {
	Base
	Type   string       `sql:""`
	Object api.Provider `sql:""`
}

func (m *Provider) With(p *api.Provider) {
	m.Base.With(p)
	m.Type = p.Type()
	m.Object = *p
}

//
// Namespace
type Namespace struct {
	Base
	Object core.Namespace `sql:""`
}

func (m *Namespace) With(n *core.Namespace) {
	m.Base.With(n)
	m.Object = *n
}

//
// StorageClass
type StorageClass struct {
	Base
	Object storage.StorageClass `sql:""`
}

func (m *StorageClass) With(s *storage.StorageClass) {
	m.Base.With(s)
	m.Object = *s
}

//
// NetworkAttachmentDefinition
type NetworkAttachmentDefinition struct {
	Base
	Object net.NetworkAttachmentDefinition `sql:""`
}

func (m *NetworkAttachmentDefinition) With(n *net.NetworkAttachmentDefinition) {
	m.Base.With(n)
	m.Object = *n
}

//
// VM
type VM struct {
	Base
	Object cnv.VirtualMachine `sql:""`
}

func (m *VM) With(v *cnv.VirtualMachine) {
	m.Base.With(v)
	m.Object = *v
}
