package ova

import (
	"net/http"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot   = ProviderRoot + "/tree"
	TreeVMRoot = TreeRoot + "/vm"
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
	// VM list.
	vm []model.VM
}

// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	//e.GET(TreeVMRoot, h.Tree)
}

// Prepare to handle the request.
func (h *TreeHandler) Prepare(ctx *gin.Context) int {
	status, err := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return status
	}
	db := h.Collector.DB()
	err = db.List(
		&h.vm,
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		return http.StatusInternalServerError
	}

	return http.StatusOK
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
	// status := h.Prepare(ctx)
	// if status != http.StatusOK {
	// 	ctx.Status(status)
	// 	return
	// }
	// if h.WatchRequest {
	// 	ctx.Status(http.StatusBadRequest)
	// 	return
	// }
	// db := h.Collector.DB()
	// pb := PathBuilder{DB: db}
	// content := TreeNode{}
	// for _, vm := range h.vm {
	// 	tr := Tree{
	// 		NodeBuilder: &NodeBuilder{
	// 			handler:     h.Handler,
	// 			pathBuilder: pb,
	// 			detail: map[string]int{
	// 				model.VmKind: h.Detail,
	// 			},
	// 		},
	// 	}
	// 	branch, err := tr.Build(
	// 		&vm,
	// 		&BranchNavigator{
	// 			detail: h.Detail,
	// 			db:     db,
	// 		})
	// 	if err != nil {
	// 		log.Trace(
	// 			err,
	// 			"url",
	// 			ctx.Request.URL)
	// 		ctx.Status(http.StatusInternalServerError)
	// 		return
	// 	}
	// 	r := VM{}
	// 	r.With(&vm)
	// 	r.Link(h.Provider)
	// 	r.Path = pb.Path(&vm)
	// 	branch.Kind = model.VmKind
	// 	branch.Object = r
	// 	content.Children = append(content.Children, branch)
	// }

	// ctx.JSON(http.StatusOK, content)
}

// Tree (branch) navigator.
type BranchNavigator struct {
}

// Tree node builder.
type NodeBuilder struct {
	// Handler.
	handler Handler
	// Resource details by kind.
	detail map[string]int
	// Path builder.
	pathBuilder PathBuilder
}

// Build a node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m model.Model) *TreeNode {
	provider := r.handler.Provider
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DiskKind:
		resource := &Disk{}
		resource.With(m.(*model.Disk))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.StorageKind:
		resource := &Storage{}
		resource.With(m.(*model.Storage))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	}

	return node
}

// Build with detail.
func (r *NodeBuilder) withDetail(kind string) int {
	if b, found := r.detail[kind]; found {
		return b
	}

	return 0
}
