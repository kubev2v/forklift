package vsphere

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"net/http"
)

//
// Routes.
const (
	TreeRoot     = ProviderRoot + "/tree"
	TreeHostRoot = TreeRoot + "/host"
	TreeVmRoot   = TreeRoot + "/vm"
)

//
// Tree handler.
type TreeHandler struct {
	Handler
	// Datacenters list.
	datacenters []model.Datacenter
}

//
// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeHostRoot, h.HostTree)
	e.GET(TreeVmRoot, h.VmTree)
}

//
// Prepare to handle the request.
func (h *TreeHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return status
	}
	db := h.Reconciler.DB()
	err := db.List(
		&h.datacenters,
		libmodel.ListOptions{
			Detail: 1,
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

//
// List not supported.
func (h TreeHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// Get not supported.
func (h TreeHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// VM Tree.
func (h TreeHandler) VmTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	db := h.Reconciler.DB()
	content := TreeNode{}
	for _, dc := range h.datacenters {
		ref := dc.Vms
		folder := &model.Folder{
			Base: model.Base{
				ID: ref.ID,
			},
		}
		err := db.Get(folder)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		navigator := func(m libmodel.Model) (refs []model.Ref) {
			switch m.(type) {
			case *model.Folder:
				refs = m.(*model.Folder).Children
			}

			return
		}
		tr := Tree{
			Provider: h.Provider,
			DB:       db,
			Detail: map[string]bool{
				model.VmKind: h.Detail,
			},
		}
		branch, err := tr.Build(folder, navigator)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Datacenter{}
		r.With(&dc)
		r.SelfLink = DatacenterHandler{}.Link(h.Provider, &dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Cluster & Host Tree.
func (h TreeHandler) HostTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	db := h.Reconciler.DB()
	content := TreeNode{}
	for _, dc := range h.datacenters {
		ref := dc.Clusters
		folder := &model.Folder{
			Base: model.Base{
				ID: ref.ID,
			},
		}
		err := db.Get(folder)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		navigator := func(m libmodel.Model) (refs []model.Ref) {
			switch m.(type) {
			case *model.Folder:
				refs = m.(*model.Folder).Children
			case *model.Cluster:
				refs = m.(*model.Cluster).Hosts
			case *model.Host:
				refs = m.(*model.Host).Vms
			}

			return
		}
		tr := Tree{
			Provider: h.Provider,
			DB:       db,
			Detail: map[string]bool{
				model.ClusterKind: h.Detail,
				model.HostKind:    h.Detail,
				model.VmKind:      h.Detail,
			},
		}
		branch, err := tr.Build(folder, navigator)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Datacenter{}
		r.With(&dc)
		r.SelfLink = DatacenterHandler{}.Link(h.Provider, &dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Tree.
type Tree struct {
	// Provider.
	Provider *api.Provider
	// DB connection.
	DB libmodel.DB
	// Depth limit.
	Depth int
	// Resource details by kind.
	Detail map[string]bool
}

//
// Build the tree
func (r *Tree) Build(m model.Model, navigator model.BranchNavigator) (*TreeNode, error) {
	root := r.node(nil, m)
	node := root
	var walk func(*model.TreeNode)
	walk = func(n *model.TreeNode) {
		child := r.node(node, n.Model)
		node.Children = append(node.Children, child)
		node = child
		defer func() {
			node = node.parent
		}()
		for _, mt := range n.Children {
			walk(mt)
		}
	}
	tree := model.Tree{
		DB:    r.DB,
		Depth: r.Depth,
	}
	modelRoot, err := tree.Build(m, navigator)
	if err != nil {
		return nil, err
	}
	for _, child := range modelRoot.Children {
		walk(child)
	}

	return root, nil
}

//
// Ancestry (Tree).
func (r *Tree) Ancestry(leaf model.Model, navigator model.ParentNavigator) (*TreeNode, error) {
	root := &TreeNode{}
	node := root
	var walk func(*model.TreeNode)
	walk = func(n *model.TreeNode) {
		child := r.node(node, n.Model)
		node.Children = append(node.Children, child)
		node = child
		defer func() {
			node = node.parent
		}()
		for _, mt := range n.Children {
			walk(mt)
		}
	}
	tree := model.Tree{
		DB:    r.DB,
		Depth: r.Depth,
	}
	modelRoot, err := tree.Ancestry(leaf, navigator)
	if err != nil {
		return nil, err
	}
	root = r.node(nil, modelRoot.Model)
	node = root
	for _, child := range modelRoot.Children {
		walk(child)
	}

	return root, nil
}

//
// Build a node for the model.
func (r *Tree) node(parent *TreeNode, m model.Model) *TreeNode {
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.FolderKind:
		resource := &Folder{}
		resource.With(m.(*model.Folder))
		resource.SelfLink =
			FolderHandler{}.Link(r.Provider, m.(*model.Folder))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DatacenterKind:
		resource := &Datacenter{}
		resource.With(m.(*model.Datacenter))
		resource.SelfLink =
			DatacenterHandler{}.Link(r.Provider, m.(*model.Datacenter))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		resource.SelfLink =
			ClusterHandler{}.Link(r.Provider, m.(*model.Cluster))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		resource.SelfLink =
			HostHandler{}.Link(r.Provider, m.(*model.Host))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.SelfLink =
			VMHandler{}.Link(r.Provider, m.(*model.VM))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.SelfLink =
			NetworkHandler{}.Link(r.Provider, m.(*model.Network))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DsKind:
		resource := &Datastore{}
		resource.With(m.(*model.Datastore))
		resource.SelfLink =
			DatastoreHandler{}.Link(r.Provider, m.(*model.Datastore))
		object := resource.Content(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	}

	return node
}

//
// Include resource details.
func (r *Tree) detail(kind string) bool {
	if b, found := r.Detail[kind]; found {
		return b
	}

	return false
}

//
// Tree node resource.
type TreeNode struct {
	// Parent node.
	parent *TreeNode
	// Object kind.
	Kind string `json:"kind"`
	// Object (resource).
	Object interface{} `json:"object"`
	// Child nodes.
	Children []*TreeNode `json:"children"`
}
