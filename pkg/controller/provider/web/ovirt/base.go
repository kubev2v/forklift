package ovirt

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	pathlib "path"
	"strings"
)

//
// Package logger.
var log = logging.WithName("web|ovirt")

//
// Fields.
const (
	DetailParam = base.DetailParam
	NameParam   = base.NameParam
)

//
// Base handler.
type Handler struct {
	base.Handler
}

//
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

//
// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := 0
	if h.Detail {
		detail = 1
	}
	return libmodel.ListOptions{
		Predicate: h.Predicate(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

//
// Path builder.
type PathBuilder struct {
	// Database.
	DB libmodel.DB
	// Cached resources.
	cache map[string]string
}

//
// Build.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch m.(type) {
	case *model.DataCenter:
		path = m.(*model.DataCenter).Name
	case *model.Cluster:
		object := m.(*model.Cluster)
		path, err = r.forDataCenter(object.DataCenter, object.Name)
	case *model.Network:
		object := m.(*model.Network)
		path, err = r.forDataCenter(object.DataCenter, object.Name)
	case *model.StorageDomain:
		object := m.(*model.StorageDomain)
		path, err = r.forDataCenter(object.DataCenter, object.Name)
	case *model.Host:
		object := m.(*model.Host)
		path, err = r.forCluster(object.Cluster, object.Name)
	case *model.VM:
		object := m.(*model.VM)
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

//
// Path based on DataCenter.
func (r *PathBuilder) forDataCenter(id, leaf string) (path string, err error) {
	name, cached := r.cache[id]
	if !cached {
		m := &model.DataCenter{
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

//
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
