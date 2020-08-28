package ocp

import (
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model/base"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"path"
)

// Errors
var NotFound = libmodel.NotFound

//
// Types
type Model = libmodel.Model
type Annotation = base.Annotation

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
	UID string `sql:""`
	// Resource version.
	Version string `sql:""`
	// Namespace.
	Namespace string `sql:"key"`
	// Name.
	Name string `sql:"key"`
	// Json encoded (raw) object.
	Object string `sql:""`
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
	m.EncodeObject(r)
}

//
// Encode (set) the object field.
func (m *Base) EncodeObject(r interface{}) {
	b, _ := json.Marshal(r)
	m.Object = string(b)
}

//
// Decode the object field.
// `r` must be pointer to the appropriate k8s object.
func (m *Base) DecodeObject(r interface{}) interface{} {
	json.Unmarshal([]byte(m.Object), r)
	return r
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

type Provider struct {
	Base
	Type string `sql:""`
}

func (m *Provider) With(p *api.Provider) {
	m.Base.With(p)
	m.Type = p.Type()
}
