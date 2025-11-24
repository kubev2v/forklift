package dynamic

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// Refresh forces an immediate inventory refresh
func (h *Handler) Refresh(ctx *gin.Context) {
	// Extract provider type and UID from URL
	providerType := ctx.Param("type")
	providerUID := ctx.Param("provider")

	// Verify this is a registered dynamic provider type
	if !h.registry.IsDynamic(providerType) {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "not a dynamic provider type",
		})
		return
	}

	// Get collector
	collector := h.getCollector(providerUID)
	if collector == nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "provider not found",
		})
		return
	}

	// Verify provider type matches URL
	provider := collector.Owner().(*api.Provider)
	if string(provider.Type()) != providerType {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "provider type mismatch",
		})
		return
	}

	// Trigger refresh by resetting collector
	collector.Reset()

	log.Info("Forced inventory refresh triggered",
		"provider", provider.Name,
		"namespace", provider.Namespace)

	ctx.JSON(http.StatusOK, gin.H{
		"message":   "inventory refresh triggered",
		"provider":  provider.Name,
		"namespace": provider.Namespace,
	})
}
