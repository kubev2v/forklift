package ocp

import (
	"errors"
	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	cnv "kubevirt.io/client-go/api/v1"
	"net/http"
)

//
// Routes.
const (
	VmParam = "vm"
	VMsRoot = NamespaceRoot + "/vms"
	VMRoot  = VMsRoot + "/:" + VmParam
)

//
// VM handler.
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
func (h VMHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.VM{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		Log.Trace(err)
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
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(VmParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
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
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			Ns2Param:           m.Namespace,
			VmParam:            m.Name,
		})
}

//
// REST Resource.
type VM struct {
	Resource
	Object cnv.VirtualMachine `json:"object"`
}

//
// Set fields with the specified object.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

//
// As content.
func (r *VM) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
