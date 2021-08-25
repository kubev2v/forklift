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
	h.Detail = true
	r := Workload{}
	r.With(m)
	err = r.Expand(h.Collector.DB())
	if err != nil {
		return
	}
	r.Link(h.Provider)

	ctx.JSON(http.StatusOK, r)
}

//
// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	VM
	Host       *Host      `json:"host"`
	Cluster    Cluster    `json:"cluster"`
	DataCenter DataCenter `json:"dataCenter"`
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
	r.Cluster.Link(p)
	r.DataCenter.Link(p)
	if r.Host != nil {
		r.Host.Link(p)
	}
}

//
// Expand the workload.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	// VM
	err = r.VM.Expand(db)
	if err != nil {
		return err
	}
	// Host
	if r.VM.Host != "" {
		r.Host = &Host{}
		host := &model.Host{
			Base: model.Base{ID: r.VM.Host},
		}
		err = db.Get(host)
		if err != nil {
			return err
		}
		r.Host.With(host)
	}
	// Cluster.
	cluster := &model.Cluster{
		Base: model.Base{ID: r.VM.Cluster},
	}
	err = db.Get(cluster)
	if err != nil {
		return err
	}
	r.Cluster.With(cluster)
	// DataCenter.
	dataCenter := &model.DataCenter{
		Base: model.Base{ID: cluster.DataCenter},
	}
	err = db.Get(dataCenter)
	if err != nil {
		return err
	}
	r.DataCenter.With(dataCenter)

	return
}
