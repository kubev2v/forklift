package vsphere

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libref "github.com/konveyor/controller/pkg/ref"
)

//
// Kinds
var (
	FolderKind     = libref.ToKind(Folder{})
	DatacenterKind = libref.ToKind(Datacenter{})
	ClusterKind    = libref.ToKind(Cluster{})
	HostKind       = libref.ToKind(Host{})
	NetKind        = libref.ToKind(Network{})
	DsKind         = libref.ToKind(Datastore{})
	VmKind         = libref.ToKind(VM{})
)

//
// Invalid reference.
type InvalidRefError struct {
	Ref
}

func (r InvalidRefError) Error() string {
	return fmt.Sprintf("Reference %#v not valid.", r.Ref)
}

//
// Invalid kind.
type InvalidKindError struct {
	Object interface{}
}

func (r InvalidKindError) Error() string {
	return fmt.Sprintf("Kind %#v not valid.", r.Object)
}

//
// Tree.
type Tree struct {
	// DB connection.
	DB libmodel.DB
	// Depth limit (0=unlimited).
	Depth int
}

//
// Build the tree.
func (r *Tree) Build(root Model, navigator BranchNavigator) (*TreeNode, error) {
	node := &TreeNode{
		Kind:  libref.ToKind(root),
		Model: root,
	}
	treeRoot := node
	depth := 0
	var walk func(Model, bool) error
	walk = func(model Model, asChild bool) error {
		kind := libref.ToKind(model)
		if asChild {
			child := &TreeNode{
				Parent: node,
				Kind:   kind,
				Model:  model,
			}
			depth++
			defer func() {
				depth--
			}()
			if r.Depth > 0 && depth > r.Depth {
				return nil
			}
			node.Children = append(node.Children, child)
			node = child
			defer func() {
				node = node.Parent
			}()
		}
		for _, ref := range navigator(model) {
			m, err := ref.Get(r.DB)
			if err != nil {
				return liberr.Wrap(err)
			}
			err = walk(m, true)
			if err != nil {
				return liberr.Wrap(err)
			}
		}

		return nil
	}
	err := walk(root, false)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return treeRoot, nil
}

//
// Build the (ancestry) tree.
func (r *Tree) Ancestry(leaf Model, navigator ParentNavigator) (*TreeNode, error) {
	node := &TreeNode{
		Kind:  libref.ToKind(leaf),
		Model: leaf,
	}
	root := node
	for {
		ref := navigator(node.Model)
		if ref.Kind == "" {
			break
		}
		m, err := ref.Get(r.DB)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		root = &TreeNode{
			Kind:  ref.Kind,
			Model: m,
		}
		root.Children = append(root.Children, node)
		node.Parent = root
		node = root
	}

	return root, nil
}

//
// Tree node.
type TreeNode struct {
	// Parent node.
	Parent *TreeNode
	// Kind of model.
	Kind string
	// Model.
	Model Model
	// Child nodes.
	Children []*TreeNode
}

//
// Tree navigator.
// Navigate up the parent tree.
type ParentNavigator func(Model) Ref

//
// Tree navigator.
// Navigate down the children.
type BranchNavigator func(Model) []Ref
