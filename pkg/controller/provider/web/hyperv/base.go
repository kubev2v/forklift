package hyperv

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Params.
const (
	ProviderParam = base.ProviderParam
	DetailParam   = base.DetailParam
	NameParam     = "name"
)

// Base handler.
type Handler struct {
	base.Handler
}

// Prepare to handle the request.
func (h *Handler) Prepare(ctx *gin.Context) int {
	status, _ := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	if h.Provider.Type() != api.HyperV {
		ctx.Status(http.StatusNotFound)
		return http.StatusNotFound
	}
	var found bool
	h.Collector, found = h.Container.Get(h.Provider)
	if !found {
		log.Trace(nil, "collector not found", "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
	if !h.Collector.HasParity() {
		ctx.Status(http.StatusServiceUnavailable)
		return http.StatusServiceUnavailable
	}

	return http.StatusOK
}

// Build list options from query parameters.
func (h *Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}
	return libmodel.ListOptions{
		Detail: detail,
		Page:   &h.Page,
	}
}

// REST Resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Variant
	Variant string `json:"variant,omitempty"`
	// Object name.
	Name string `json:"name"`
	// Revision
	Revision int64 `json:"revision"`
	// Self URI.
	SelfLink string `json:"selfLink"`
	// Path
	Path string `json:"path,omitempty"`
}

// Build the resource with the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Name = m.Name
	r.Revision = m.Revision
}
