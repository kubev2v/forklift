package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
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
	tr := Tree{
		Provider: h.Provider,
		DB:       db,
		Detail: map[string]bool{
			model.DatacenterKind: true,
			model.ClusterKind:    true,
			model.HostKind:       true,
			model.VmKind:         true,
		},
	}
	navigator := func(m libmodel.Model) (ref model.Ref) {
		switch m.(type) {
		case *model.Folder:
			ref = m.(*model.Folder).Parent
		case *model.Host:
			ref = m.(*model.Host).Parent
		case *model.Cluster:
			ref = m.(*model.Cluster).Parent
		case *model.VM:
			ref = m.(*model.VM).Host
		}

		return
	}
	root, err := tr.Ancestry(m, navigator)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := Workload{}
	content.SelfLink = h.Link(h.Provider, m)
	content.With(root)

	ctx.JSON(http.StatusOK, content)
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
// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	*VM
	Host struct {
		*Host
		Cluster struct {
			*Cluster
			Datacenter *Datacenter `json:"datacenter"`
		} `json:"cluster"`
	} `json:"host"`
}

func (r *Workload) With(root *TreeNode) {
	node := root
	for {
		switch node.Kind {
		case model.DatacenterKind:
			r.Host.Cluster.Datacenter = node.Object.(*Datacenter)
		case model.ClusterKind:
			r.Host.Cluster.Cluster = node.Object.(*Cluster)
		case model.HostKind:
			r.Host.Host = node.Object.(*Host)
		case model.VmKind:
			r.VM = node.Object.(*VM)
		}
		if len(node.Children) > 0 {
			node = node.Children[0]
		} else {
			break
		}
	}
}
