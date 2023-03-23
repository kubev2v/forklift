package openstack

import (
	"net/http"

	"github.com/gin-gonic/gin"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot        = ProviderRoot + "/tree"
	TreeProjectRoot = TreeRoot + "/project"
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
	// Project list.
	projects []model.Project
}

// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeProjectRoot, h.Tree)
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
		&h.projects,
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
	for _, project := range h.projects {
		tr := Tree{
			NodeBuilder: &NodeBuilder{
				handler:     h.Handler,
				pathBuilder: pb,
				detail: map[string]int{
					model.VMKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			&project,
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
		r := Project{}
		r.With(&project)
		r.Link(h.Provider)
		r.Path = pb.Path(&project)
		branch.Kind = model.ProjectKind
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
func (n *BranchNavigator) Next(p libmodel.Model) (r []model.Model, err error) {
	switch p.(type) {
	case *model.Project:
		m := p.(*model.Project)
		vmList, nErr := n.listVM(m)
		if nErr == nil {
			for i := range vmList {
				m := &vmList[i]
				r = append(r, m)
			}
		} else {
			err = nErr
		}
	}

	return
}

func (n *BranchNavigator) listVM(p *model.Project) (list []model.VM, err error) {
	detail := 0
	if n.detail > 0 {
		detail = model.MaxDetail
	}
	list = []model.VM{}
	err = n.db.List(
		&list,
		model.ListOptions{
			Predicate: libmodel.Eq("TenantID", p.ID),
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
	case model.RegionKind:
		resource := &Region{}
		resource.With(m.(*model.Region))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ProjectKind:
		resource := &Project{}
		resource.With(m.(*model.Project))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ImageKind:
		resource := &Image{}
		resource.With(m.(*model.Image))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.FlavorKind:
		resource := &Flavor{}
		resource.With(m.(*model.Flavor))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VMKind:
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
	case model.SnapshotKind:
		resource := &Snapshot{}
		resource.With(m.(*model.Snapshot))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VolumeKind:
		resource := &Volume{}
		resource.With(m.(*model.Volume))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VolumeTypeKind:
		resource := &VolumeType{}
		resource.With(m.(*model.VolumeType))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetworkKind:
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
	case model.SubnetKind:
		resource := &Subnet{}
		resource.With(m.(*model.Subnet))
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
