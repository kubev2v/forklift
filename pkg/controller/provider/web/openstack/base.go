package openstack

import (
	"strings"

	"github.com/gin-gonic/gin"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	pathlib "path"
)

// Package logger.
var log = logging.WithName("web|openstack")

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

// Build.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch m.(type) {
	case *model.Project:
		path = m.(*model.Project).Name
	case *model.VM:
		object := m.(*model.VM)
		path, err = r.forProject(object.TenantID, object.Name)
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

// Path based on Project.
func (r *PathBuilder) forProject(id, leaf string) (path string, err error) {
	name, cached := r.cache[id]
	if !cached {
		m := &model.Project{
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
