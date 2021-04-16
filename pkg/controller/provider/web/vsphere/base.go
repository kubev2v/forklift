package vsphere

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"strings"
)

//
// Package logger.
var log = logging.WithName("web|vsphere")

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
// Match (compare) paths.
// Determine if the relative path is contained
// in the absolute path.
func (h Handler) PathMatch(absolute, relative string) (matched bool) {
	absolute = strings.TrimLeft(absolute, "/")
	relative = strings.TrimLeft(relative, "/")
	pathA := strings.Split(absolute, "/")
	pathR := strings.Split(relative, "/")
	a := len(pathA) - 1
	r := len(pathR) - 1
	for {
		if r < 0 {
			matched = true
			break
		}
		if a < 0 {
			break
		}
		if pathA[a] != pathR[r] {
			break
		}
		a--
		r--
	}
	return
}

//
// Match (compare) paths.
// Determine if the paths have the same root.
func (h Handler) PathMatchRoot(absolute, path string) (matched bool) {
	absolute = strings.TrimLeft(absolute, "/")
	path = strings.TrimLeft(path, "/")
	dcA := strings.Split(absolute, "/")[0]
	dcB := strings.Split(path, "/")[0]
	matched = dcA == dcB
	return
}
