package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
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
	h.Detail = true
	r := Workload{}
	err = h.Build(m, &r)
	r.SelfLink = h.Link(h.Provider, m)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, r)
}

//
// Build self link (URI).
func (h WorkloadHandler) Link(p *api.Provider, m *model.VM) string {
	return h.Handler.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            m.ID,
		})
}

//
// Build the workload.
func (h *WorkloadHandler) Build(m *model.VM, r *Workload) (err error) {
	db := h.Reconciler.DB()
	// VM
	vh := VMHandler{Handler: h.Handler}
	err = vh.Build(m, &r.VM)
	if err != nil {
		return err
	}
	// Host
	if m.Host == "" {
		return
	}
	host := &model.Host{
		Base: model.Base{ID: m.Host},
	}
	err = db.Get(host)
	if err != nil {
		return err
	}
	r.Host.Host.With(host)
	r.Host.Host.SelfLink = HostHandler{}.Link(h.Provider, host)
	// Cluster.
	cluster := &model.Cluster{
		Base: model.Base{ID: host.Cluster},
	}
	err = db.Get(cluster)
	if err != nil {
		return err
	}
	r.Host.Cluster.With(cluster)
	r.Host.Cluster.SelfLink = ClusterHandler{}.Link(h.Provider, cluster)
	// DataCenter.
	dataCenter := &model.DataCenter{
		Base: model.Base{ID: cluster.DataCenter},
	}
	err = db.Get(dataCenter)
	if err != nil {
		return err
	}
	r.Host.Cluster.DataCenter.With(dataCenter)
	r.Host.Cluster.DataCenter.SelfLink = DataCenterHandler{}.Link(h.Provider, dataCenter)

	return
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
			DataCenter DataCenter `json:"dataCenter"`
		} `json:"cluster"`
	} `json:"host"`
}
