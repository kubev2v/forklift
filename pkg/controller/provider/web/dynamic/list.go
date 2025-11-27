package dynamic

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
)

// List all providers of a specific dynamic type
func (h *Handler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	content, err := h.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

// ListContent builds the list of providers for a specific dynamic type
func (h *Handler) ListContent(ctx *gin.Context) (content []interface{}, err error) {
	content = []interface{}{}

	// Get provider type from URL
	providerType := ctx.Param("type")

	// Verify this is a registered dynamic provider type
	if !h.registry.IsDynamic(providerType) {
		// Not a dynamic provider type, return empty list
		return
	}

	// Get optional namespace filter from query
	q := ctx.Request.URL.Query()
	ns := q.Get(base.NsParam)

	// Iterate through all collectors
	list := h.Container.List()
	for _, collector := range list {
		if p, cast := collector.Owner().(*api.Provider); cast {
			// Filter by provider type
			if string(p.Type()) != providerType {
				continue
			}
			// Filter by namespace if specified
			if ns != "" && ns != p.Namespace {
				continue
			}

			// Build provider representation
			provider := map[string]interface{}{
				"uid":       string(p.UID),
				"version":   p.ResourceVersion,
				"namespace": p.Namespace,
				"name":      p.Name,
				"type":      string(p.Type()),
				"object":    p,
			}

			content = append(content, provider)
		}
	}

	// Apply pagination
	h.Page.Slice(&content)

	return
}
