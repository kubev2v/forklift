package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	WorkloadCollection = "workloads"
	WorkloadsRoot      = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

//
// Virtual Machine handler.
type WorkloadHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	e.GET(WorkloadRoot, h.Get)
}

//
// List resources in a REST collection.
func (h WorkloadHandler) List(ctx *gin.Context) {
}

//
// Get a specific REST resource.
func (h WorkloadHandler) Get(ctx *gin.Context) {
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
	db := h.Collector.DB()
	err := db.Get(m)
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
	err = r.Expand(db)
	if err != nil {
		return
	}
	r.Link(h.Provider)
	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	VM
	Host struct {
		Host
		Cluster struct {
			Cluster
			Datacenter *Datacenter `json:"datacenter"`
		} `json:"cluster"`
	} `json:"host"`
}

func (r *Workload) With(m *model.VM) {
	r.VM.With(m)
	r.Host.ID = m.Host
}

//
// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
	r.Host.Link(p)
	r.Host.Cluster.Link(p)
}

//
// Expand the resource.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	host := &model.Host{
		Base: model.Base{ID: r.Host.ID},
	}
	err = db.Get(host)
	if err != nil {
		return
	}
	r.Host.Host.With(host)
	cluster := &model.Cluster{
		Base: model.Base{ID: host.Cluster},
	}
	err = db.Get(cluster)
	if err != nil {
		return
	}
	r.Host.Cluster.Cluster.With(cluster)

	return
}
