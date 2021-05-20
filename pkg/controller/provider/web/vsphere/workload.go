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
		NodeBuilder: &NodeBuilder{
			provider: h.Provider,
			detail: map[string]bool{
				model.DatacenterKind: true,
				model.ClusterKind:    true,
				model.HostKind:       true,
				model.VmKind:         true,
			},
		},
	}
	root, err := tr.Ancestry(m, &WorkloadNavigator{db: db})
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
// Workload navigator.
type WorkloadNavigator struct {
	db libmodel.DB
}

//
// Next parent.
func (n *WorkloadNavigator) Next(m model.Model) (r model.Model, err error) {
	switch m.(type) {
	case *model.Host:
		m := &model.Cluster{
			Base: model.Base{
				ID: m.(*model.Host).Parent.ID,
			},
		}
		err = n.db.Get(m)
		if err == nil {
			r = m
		}
	case *model.VM:
		m := &model.Host{
			Base: model.Base{
				ID: m.(*model.VM).Host,
			},
		}
		err = n.db.Get(m)
		if err == nil {
			r = m
		}
	}

	return
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
