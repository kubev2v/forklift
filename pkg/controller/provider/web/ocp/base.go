package ocp

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
)

//
// Base handler.
type Handler struct {
	base.Handler
}

//
// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := 0
	if h.Detail {
		detail = 1
	}
	return libmodel.ListOptions{
		Detail: detail,
		Page:   &h.Page,
	}
}
