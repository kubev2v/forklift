package ocp

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
)

//
// Package logger.
var log = logging.WithName("web|ocp")

//
// Params.
const (
	NsParam     = base.NsParam
	NameParam   = base.NameParam
	DetailParam = base.DetailParam
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
	ns := q.Get(NsParam)
	and := libmodel.And()
	if len(ns) > 0 {
		and.Predicates = append(
			and.Predicates,
			libmodel.Eq(NsParam, ns))
	}
	name := q.Get(NameParam)
	if len(name) > 0 {
		and.Predicates = append(
			and.Predicates,
			libmodel.Eq(NameParam, name))
	}
	switch len(and.Predicates) {
	case 0: // All.
	case 1:
		p = and.Predicates[0]
	default:
		p = and
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
