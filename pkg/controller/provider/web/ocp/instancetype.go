package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
)

// Routes.
const (
	InstanceParam = "instancetype"
	InstancesRoot = ProviderRoot + "/instancetypes"
	InstanceRoot  = InstancesRoot + "/:" + InstanceParam
)

// InstanceType handler.
type InstanceHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *InstanceHandler) AddRoutes(e *gin.Engine) {
	e.GET(InstancesRoot, h.List)
	e.GET(InstancesRoot+"/", h.List)
	e.GET(InstanceRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h InstanceHandler) List(ctx *gin.Context) {
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
	instancetypes, err := h.InstanceTypes(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range instancetypes {
		r := &InstanceType{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h InstanceHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	instancetypes, err := h.InstanceTypes(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range instancetypes {
		if ctx.Param(InstanceParam) == m.UID {
			r := &InstanceType{}
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
type InstanceType struct {
	Resource
	Object instancetype.VirtualMachineInstancetype `json:"object"`
}

// Set fields with the specified object.
func (r *InstanceType) With(m *model.InstanceType) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *InstanceType) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		InstanceRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			InstanceRoot:       r.UID,
		})
}

// As content.
func (r *InstanceType) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
