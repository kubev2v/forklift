package vsphere

import (
	"strings"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("web|vsphere")

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
	// Cached names by ID.
	cache map[model.Ref]*model.Base
}

// Determine object path.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[model.Ref]*model.Base{}
	}
	parts := []string{m.GetName()}
	node := m
Walk:
	for {
		parent := node.GetParent()
		switch parent.Kind {
		case model.FolderKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Folder{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			if b.GetParent().Kind == "" {
				break Walk
			}
			parts = append(parts, b.Name)
			node = b
		case model.DatacenterKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Datacenter{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			parts = append(parts, b.Name)
			node = b
		case model.ClusterKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Cluster{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			parts = append(parts, b.Name)
			node = b
		case model.HostKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Host{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			parts = append(parts, b.Name)
			node = b
		case model.NetKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Network{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			parts = append(parts, b.Name)
			node = b
		case model.DsKind:
			b, cached := r.cache[parent]
			if !cached {
				m := &model.Datastore{}
				m.WithRef(parent)
				err = r.DB.Get(m)
				if err != nil {
					return
				}
				b = &m.Base
				r.cache[parent] = b
			}
			parts = append(parts, b.Name)
			node = b
		default:
			break Walk
		}
	}

	reversed := []string{""}
	for i := len(parts) - 1; i >= 0; i-- {
		reversed = append(reversed, parts[i])
	}

	path = strings.Join(reversed, "/")

	return
}
