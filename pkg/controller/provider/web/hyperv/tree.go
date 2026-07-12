package hyperv

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot     = ProviderRoot + "/tree"
	TreeHostRoot = TreeRoot + "/host"
)

// Types.
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
}

func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeHostRoot, h.HostTree)
}

// HostTree builds a Cluster → Host → VM tree for clustered providers,
// or an empty response for standalone providers.
func (h TreeHandler) HostTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Collector.DB()
	clusters := []model.Cluster{}
	if err := db.List(&clusters, libmodel.ListOptions{}); err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	if len(clusters) == 0 {
		ctx.JSON(http.StatusOK, []interface{}{})
		return
	}
	nb := &NodeBuilder{
		provider: h.Provider,
		detail:   map[string]int{},
		db:       db,
	}
	var roots []*TreeNode
	for i := range clusters {
		clusterNode := nb.Node(nil, &clusters[i])
		hosts := []model.Host{}
		err := db.List(&hosts, libmodel.ListOptions{
			Predicate: libmodel.Eq("cluster", clusters[i].Pk()),
		})
		if err != nil {
			log.Trace(err, "url", ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		for j := range hosts {
			hostNode := nb.Node(clusterNode, &hosts[j])
			clusterNode.Children = append(clusterNode.Children, hostNode)
			vms := []model.VM{}
			err := db.List(&vms, libmodel.ListOptions{
				Predicate: libmodel.Eq("host", hosts[j].Name),
			})
			if err != nil {
				log.Trace(err, "url", ctx.Request.URL)
				ctx.Status(http.StatusInternalServerError)
				return
			}
			for k := range vms {
				vmNode := nb.Node(hostNode, &vms[k])
				hostNode.Children = append(hostNode.Children, vmNode)
			}
		}
		roots = append(roots, clusterNode)
	}
	ctx.JSON(http.StatusOK, roots)
}

// Tree node builder.
type NodeBuilder struct {
	provider *api.Provider
	detail   map[string]int
	db       libmodel.DB
}

// Node builds a tree node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m libmodel.Model) *TreeNode {
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DiskKind:
		resource := &Disk{}
		resource.With(m.(*model.Disk))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.StorageKind:
		resource := &Storage{}
		resource.With(m.(*model.Storage))
		resource.Link(r.provider)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	}
	return node
}

func (r *NodeBuilder) withDetail(kind string) int {
	if b, found := r.detail[kind]; found {
		return b
	}
	return 0
}
