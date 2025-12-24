package ovfbase

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	WorkloadCollection = "workloads"
)

// Virtual Machine handler.
type WorkloadHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	root := h.Config.ProviderRoot() + "/" + WorkloadCollection
	e.GET(root+"/:"+VMParam, h.Get)
}

// List resources in a REST collection.
func (h WorkloadHandler) List(ctx *gin.Context) {
}

// Get a specific REST resource.
func (h WorkloadHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	if err != nil {
		return
	}
	r := Workload{}
	r.With(m)
	r.Config = h.Config
	err = r.Expand(db)
	if err != nil {
		return
	}
	r.Link(h.Provider)
	content := r

	ctx.JSON(http.StatusOK, content)
}

// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	VM
	Config Config `json:"-"`
}

func (r *Workload) With(m *model.VM) {
	r.VM.With(m)
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		r.Config.ProviderRoot()+"/"+WorkloadCollection+"/:"+VMParam,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

// Expand the resource.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	vm := &model.VM{
		Base: model.Base{ID: r.ID},
	}
	err = db.Get(vm)
	return
}
