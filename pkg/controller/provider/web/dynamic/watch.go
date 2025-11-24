package dynamic

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// watch handles websocket watch requests for dynamic providers
// Watch requests are served from the SQLite cache, not proxied to external service
func (h *Handler) watch(ctx *gin.Context, providerUID, path string) {
	// Get collector by provider UID
	collector := h.getCollector(providerUID)
	if collector == nil {
		ctx.Status(http.StatusNotFound)
		ctx.Header(base.ReasonHeader, base.UnknownProvider)
		return
	}

	db := collector.DB()

	// Determine resource type from path
	// path format: "/vms", "/networks", "/storages"
	resourceType := strings.TrimPrefix(path, "/")
	resourceType = strings.Split(resourceType, "/")[0] // Handle "/vms/123" -> "vms"

	var err error

	switch resourceType {
	case "vms":
		// Watch VMs from SQLite cache
		err = h.Watch(
			ctx,
			db,
			&model.VM{},
			func(in libmodel.Model) (r interface{}) {
				m := in.(*model.VM)
				// Return the VM with its JSON object
				obj, _ := m.GetObject()
				r = obj
				return
			})
	case "networks":
		// Watch Networks from SQLite cache
		err = h.Watch(
			ctx,
			db,
			&model.Network{},
			func(in libmodel.Model) (r interface{}) {
				m := in.(*model.Network)
				obj, _ := m.GetObject()
				r = obj
				return
			})
	case "storage":
		// Watch Storage from SQLite cache
		err = h.Watch(
			ctx,
			db,
			&model.Storage{},
			func(in libmodel.Model) (r interface{}) {
				m := in.(*model.Storage)
				obj, _ := m.GetObject()
				r = obj
				return
			})
	case "disks":
		// Watch Disks from SQLite cache
		err = h.Watch(
			ctx,
			db,
			&model.Disk{},
			func(in libmodel.Model) (r interface{}) {
				m := in.(*model.Disk)
				obj, _ := m.GetObject()
				r = obj
				return
			})
	default:
		// Unsupported resource type for watch
		log.V(3).Info("Watch not supported for resource type",
			"resource", resourceType,
			"provider", providerUID)
		ctx.JSON(http.StatusNotImplemented, gin.H{
			"error":     "watch not supported for this resource type",
			"resource":  resourceType,
			"supported": []string{"vms", "networks", "storage", "disks"},
		})
		return
	}

	if err != nil {
		log.Trace(err, "watch failed",
			"url", ctx.Request.URL,
			"resource", resourceType)
		ctx.Status(http.StatusInternalServerError)
	}
}
