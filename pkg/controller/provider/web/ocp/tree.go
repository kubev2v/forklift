package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot          = ProviderRoot + "/tree"
	TreeNamespaceRoot = TreeRoot + "/namespace"
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
	// Namespaces list.
	namespaces []model.Namespace
}

// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeNamespaceRoot, h.Tree)
}

// List not supported.
func (h TreeHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

// Get not supported.
func (h TreeHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
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
	vms := []model.VM{}
	err = db.List(
		&vms,
		model.ListOptions{
			Detail: libmodel.DefaultDetail,
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		return http.StatusInternalServerError
	}

	namespaceSet := make(map[string]struct{})

	for _, vm := range vms {
		namespaceSet[vm.Namespace] = struct{}{}
	}

	err = db.List(
		&h.namespaces,
		model.ListOptions{
			Detail: libmodel.DefaultDetail,
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		return http.StatusInternalServerError
	}

	filteredNamespaces := []model.Namespace{}
	for _, ns := range h.namespaces {
		if _, ok := namespaceSet[ns.Name]; !ok {
			continue
		}

		filteredNamespaces = append(filteredNamespaces, ns)
	}

	h.namespaces = filteredNamespaces

	return http.StatusOK
}

// Tree.
func (h TreeHandler) Tree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	content := TreeNode{}
	for _, ns := range h.namespaces {
		tr := Tree{
			NodeBuilder: &NodeBuilder{
				handler:     h.Handler,
				pathBuilder: pb,
				detail: map[string]int{
					model.VmKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			&ns,
			&BranchNavigator{
				detail: h.Detail,
				db:     db,
			})
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Namespace{}
		r.With(&ns)
		r.Link(h.Provider)
		r.Path = pb.Path(&ns)
		branch.Kind = model.NamespaceKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

// Tree (branch) navigator.
type BranchNavigator struct {
	db     libmodel.DB
	detail int
}

// Next (children) on the branch.
func (n *BranchNavigator) Next(p libmodel.Model) ([]model.Model, error) {
	switch ns := p.(type) {
	case *model.Namespace:
		vmList, err := n.listVM(ns.Name)
		if err != nil {
			return nil, err
		}

		models := make([]model.Model, len(vmList))
		for i := range vmList {
			models[i] = &vmList[i]
		}

		return models, nil
	}

	return nil, nil
}

func (n *BranchNavigator) listVM(namespace string) (list []model.VM, err error) {
	detail := 0
	if n.detail > 0 {
		detail = model.MaxDetail
	}

	list = []model.VM{}
	err = n.db.List(
		&list,
		model.ListOptions{
			Predicate: libmodel.Eq("Namespace", namespace),
			Detail:    detail,
		})

	return
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
	case model.NamespaceKind:
		resource := &Namespace{}
		resource.With(m.(*model.Namespace))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
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
