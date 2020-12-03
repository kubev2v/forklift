package vsphere

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
)

//
// Fields.
const (
	NameParam = "name"
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
	value := q.Get(NameParam)
	if len(value) > 0 {
		p = libmodel.Eq(NameParam, value)
	}

	return
}
