package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	cnv "kubevirt.io/api/core/v1"
)

// Routes.
const (
	VmParam = "vm"
	VMsRoot = ProviderRoot + "/vms"
	VMRoot  = VMsRoot + "/:" + VmParam
)

// VM handler.
type VMHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VMHandler) List(ctx *gin.Context) {
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
	vms, err := h.VMs(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	content := []interface{}{}
	for _, m := range vms {
		r := &VM{}
		r.With(m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	vms, err := h.VMs(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	for _, m := range vms {
		if m.UID == ctx.Param(VmParam) {
			r := &VM{}
			r.With(m)
			r.Link(h.Provider)
			content := r.Content(model.MaxDetail)
			ctx.JSON(http.StatusOK, content)
			return
		}
	}
	ctx.Status(http.StatusNotFound)
}

// REST Resource.
type VM struct {
	Resource
	Object   cnv.VirtualMachine          `json:"object"`
	Instance *cnv.VirtualMachineInstance `json:"instance,omitempty"`
}

// Set fields with the specified object.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
	r.Instance = m.Instance
}

// Build self link (URI).
func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VmParam:            r.UID,
		})
}

// As content.
func (r *VM) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
