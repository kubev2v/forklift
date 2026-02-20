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

// Types.
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
}

// HyperV has a flat topology,
// so tree endpoints are not registered
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
}

// List not supported.
func (h TreeHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

// Get not supported.
func (h TreeHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

// Tree.
func (h TreeHandler) Tree(ctx *gin.Context) {
	// Not implemented — HyperV has a flat topology.
}

// Tree node builder.
type NodeBuilder struct {
	// Provider.
	provider *api.Provider
	// Resource details by kind.
	detail map[string]int
}

// Node builds a tree node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m libmodel.Model) *TreeNode {
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
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

// withDetail returns the detail level for a kind.
func (r *NodeBuilder) withDetail(kind string) int {
	if b, found := r.detail[kind]; found {
		return b
	}
	return 0
}
