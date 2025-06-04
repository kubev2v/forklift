package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
)

// Routes.
const (
	ClusterInstanceParam = "clusterinstancetype"
	ClusterInstancesRoot = ProviderRoot + "/clusterinstancetypes"
	ClusterInstanceRoot  = ClusterInstancesRoot + "/:" + ClusterInstanceParam
)

// ClusterInstanceType handler.
type ClusterInstanceHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ClusterInstanceHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClusterInstancesRoot, h.List)
	e.GET(ClusterInstancesRoot+"/", h.List)
	e.GET(ClusterInstanceRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ClusterInstanceHandler) List(ctx *gin.Context) {
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
	clusterinstances, err := h.ClusterInstanceTypes(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	content := []interface{}{}
	for _, m := range clusterinstances {
		r := &ClusterInstanceType{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ClusterInstanceHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	clusterinstances, err := h.ClusterInstanceTypes(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range clusterinstances {
		if m.UID == ctx.Param(ClusterInstanceParam) {
			r := &ClusterInstanceType{}
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
type ClusterInstanceType struct {
	Resource
	Object instancetype.VirtualMachineClusterInstancetype `json:"object"`
}

// Set fields with the specified object.
func (r *ClusterInstanceType) With(m *model.ClusterInstanceType) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *ClusterInstanceType) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ClusterInstanceRoot,
		base.Params{
			base.ProviderParam:  string(p.UID),
			ClusterInstanceRoot: r.UID,
		})
}

// As content.
func (r *ClusterInstanceType) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
