package base

import (
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
	"time"
)

var log = logging.WithName("web|tree")

//
// Node builder.
type NodeBuilder interface {
	Node(p *TreeNode, m libmodel.Model) *TreeNode
}

//
// Tree.
type Tree struct {
	NodeBuilder
	// Depth limit.
	Depth int
}

//
// Build the tree
func (r *Tree) Build(m model.Model, navigator model.BranchNavigator) (*TreeNode, error) {
	root := r.Node(nil, m)
	node := root
	var walk func(*model.TreeNode)
	walk = func(n *model.TreeNode) {
		child := r.Node(node, n.Model)
		node.Children = append(node.Children, child)
		node = child
		defer func() {
			node = node.Parent
		}()
		for _, mt := range n.Children {
			walk(mt)
		}
	}
	tree := model.Tree{
		Depth: r.Depth,
	}
	mark := time.Now()
	modelRoot, err := tree.Build(m, navigator)
	if err != nil {
		return nil, err
	}
	for _, child := range modelRoot.Children {
		walk(child)
	}

	log.V(1).Info("Tree built.", "duration", time.Since(mark))

	return root, nil
}

//
// Ancestry (Tree).
func (r *Tree) Ancestry(leaf model.Model, navigator model.ParentNavigator) (*TreeNode, error) {
	root := &TreeNode{}
	node := root
	var walk func(*model.TreeNode)
	walk = func(n *model.TreeNode) {
		child := r.Node(node, n.Model)
		node.Children = append(node.Children, child)
		node = child
		defer func() {
			node = node.Parent
		}()
		for _, mt := range n.Children {
			walk(mt)
		}
	}
	tree := model.Tree{
		Depth: r.Depth,
	}
	modelRoot, err := tree.Ancestry(leaf, navigator)
	if err != nil {
		return nil, err
	}
	root = r.Node(nil, modelRoot.Model)
	node = root
	for _, child := range modelRoot.Children {
		walk(child)
	}

	return root, nil
}

//
// Tree node resource.
type TreeNode struct {
	// Parent node.
	Parent *TreeNode `json:"-"`
	// Object kind.
	Kind string `json:"kind"`
	// Object (resource).
	Object interface{} `json:"object"`
	// Child nodes.
	Children []*TreeNode `json:"children"`
}
