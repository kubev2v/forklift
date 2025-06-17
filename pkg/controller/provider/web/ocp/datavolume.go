package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Routes.
const (
	DataVolumeParam = "dv"
	DataVolumesRoot = ProviderRoot + "/datavolumes"
	DataVolumeRoot  = DataVolumesRoot + "/:" + DataVolumeParam
)

// DataVolume handler.
type DataVolumeHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DataVolumeHandler) AddRoutes(e *gin.Engine) {
	e.GET(DataVolumesRoot, h.List)
	e.GET(DataVolumesRoot+"/", h.List)
	e.GET(DataVolumeRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DataVolumeHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusNotImplemented)
		return
	}
	dvs, err := h.DataVolumes(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	content := []interface{}{}
	for _, m := range dvs {
		r := &DataVolume{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DataVolumeHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	dvs, err := h.DataVolumes(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range dvs {
		if m.UID == ctx.Param(DataVolumeParam) {
			r := &DataVolume{}
			r.With(&m)
			r.Link(h.Provider)
			content := r.Content(model.MaxDetail)
			ctx.JSON(http.StatusOK, content)
			return
		}
	}
	ctx.Status(http.StatusNotFound)
}

// REST Resource.
type DataVolume struct {
	Resource
	Object cdi.DataVolume `json:"object"`
}

// Set fields with the specified object.
func (r *DataVolume) With(m *model.DataVolume) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *DataVolume) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DataVolumeRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DataVolumeParam:    r.UID,
		})
}

// As content.
func (r *DataVolume) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
