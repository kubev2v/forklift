package ovfbase

import (
	"strings"

	pathlib "path"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("web|ovfbase")

// Fields.
const (
	DetailParam = base.DetailParam
	NameParam   = base.NameParam
)

// Base handler for OVF-based providers.
type Handler struct {
	base.Handler
	Config Config
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
	// Cached resource
	cache map[string]string
}

func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch m := m.(type) {
	case *model.VM:
		path = pathlib.Join(m.UUID)
	case *model.Network:
		path = pathlib.Join(m.ID)
	case *model.Disk:
		path = pathlib.Join(m.ID)
	case *model.Storage:
		path = pathlib.Join(m.ID)
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
