package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	VMParam      = "vm"
	VMCollection = "vms"
	VMsRoot      = ProviderRoot + "/" + VMCollection
	VMRoot       = VMsRoot + "/:" + VMParam
)

//
// Virtual Machine handler.
type VMHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VMHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Reconciler.DB()
	list := []model.VM{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &VM{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &VM{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h VMHandler) Link(p *api.Provider, m *model.VM) string {
	return h.Handler.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            m.ID,
		})
}

//
// Watch.
func (h VMHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VM{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.VM)
			vm := &VM{}
			vm.With(m)
			vm.SelfLink = h.Link(h.Provider, m)
			r = vm
			return
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
// REST Resource.
type VM struct {
	Resource
}

//
// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
}

//
// As content.
func (r *VM) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
