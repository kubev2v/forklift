package nutanix

import (
	pathlib "path"
	"strings"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("web|nutanix")

// Fields.
const (
	DetailParam = base.DetailParam
	NameParam   = base.NameParam
)

// Base handler.
type Handler struct {
	base.Handler
}

// Build list predicate.
func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) > 0 {
		path := strings.Split(name, "/")
		name := path[len(path)-1]
		p = libmodel.Eq(NameParam, name)
	}

	return
}

// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}
	return libmodel.ListOptions{
		Predicate: h.Predicate(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

// Path builder.
type PathBuilder struct {
	// Database.
	DB libmodel.DB
	// Cached resources.
	cache map[string]string
}

// Build path for a model.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch obj := m.(type) {
	case *model.Cluster:
		path = obj.Name
	case *model.Host:
		object := obj
		path, err = r.forCluster(object.Cluster, object.Name)
	case *model.Network:
		object := obj
		path, err = r.forCluster(object.Cluster, object.Name)
	case *model.StorageContainer:
		object := obj
		path, err = r.forCluster(object.Cluster, object.Name)
	case *model.Image:
		path = obj.Name
	case *model.VM:
		object := obj
		path, err = r.forCluster(object.Cluster, object.Name)
	}

	if err != nil {
		log.Error(
			err,
			"path builder failed.",
			"model",
			libmodel.Describe(m))
	}

	return
}

// Path based on Cluster.
func (r *PathBuilder) forCluster(id, leaf string) (path string, err error) {
	name, cached := r.cache[id]
	if !cached {
		m := &model.Cluster{
			Base: model.Base{ID: id},
		}
		err = r.DB.Get(m)
		if err != nil {
			return
		}
		name = m.Name
		r.cache[id] = name
	}

	path = pathlib.Join(name, leaf)

	return
}

// REST Resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Revision
	Revision int64 `json:"revision"`
	// Path
	Path string `json:"path,omitempty"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Name = m.Name
	r.Revision = m.Revision
}
