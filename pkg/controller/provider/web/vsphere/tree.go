package web

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/ref"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
)

//
// Tree.
type Tree struct {
	// DB connection.
	DB libmodel.DB
	// Tree root.
	Root model.Model
	// Leaf kind.
	Leaf string
	// Flatten the tree (root & leafs).
	Flatten bool
	// Depth limit.
	Depth int
	// Resource details by kind.
	Detail map[string]bool
}

//
// Build the tree
func (r *Tree) Build() (*TreeNode, error) {
	root := r.node(nil, r.Root)
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
		DB:      r.DB,
		Root:    r.Root,
		Leaf:    r.Leaf,
		Flatten: r.Flatten,
		Depth:   r.Depth,
	}
	modelRoot, err := tree.Build()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, child := range modelRoot.Children {
		walk(child)
	}

	return root, nil
}

//
// Build a node for the model.
func (r *Tree) node(parent *TreeNode, m model.Model) *TreeNode {
	kind := ref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.FolderKind:
		resource := &Folder{}
		resource.With(m.(*model.Folder))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DatacenterKind:
		resource := &Datacenter{}
		resource.With(m.(*model.Datacenter))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		object := resource.Object(r.detail(kind))
		node = &TreeNode{
			parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DsKind:
		resource := &Datastore{}
		resource.With(m.(*model.Datastore))
		object := resource.Object(r.detail(kind))
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
