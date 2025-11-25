package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Package logger.
var log = logging.WithName("ec2|web")

// Query parameters
const (
	NameParam = base.NameParam
)

// Handler base.
type Handler struct {
	base.Handler
}

// Build predicate from query parameters
func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) > 0 {
		// Handle path-based names (e.g., "vpc/name")
		path := strings.Split(name, "/")
		name = path[len(path)-1]
		p = libmodel.Eq(NameParam, name)
	}

	return
}

// Build list options from query parameters and handler state
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

// Provider handler.
type ProviderHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot, h.Get)
}

// Get provider info.
func (h *ProviderHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}
