package ova

import (
	"strings"

	pathlib "path"

	"github.com/gin-gonic/gin"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
)

// Package logger.
var log = logging.WithName("web|ova")

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
	// Cached resource
	cache map[string]string
}

func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch m.(type) {
	case *model.VM:
		vm := m.(*model.VM)
		path = pathlib.Join(vm.UUID)
	case *model.Network:
		net := m.(*model.Network)
		path = pathlib.Join(net.ID)
	case *model.Disk:
		disk := m.(*model.Disk)
		path = pathlib.Join(disk.ID)
	case *model.Storage:
		storage := m.(*model.Storage)
		path = pathlib.Join(storage.ID)
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
