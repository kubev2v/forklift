package base

import (
	libref "github.com/konveyor/controller/pkg/ref"
)

//
// Tree.
type Tree struct {
	// Depth limit (0=unlimited).
	Depth int
}

//
// Build the tree.
func (r *Tree) Build(root Model, navigator BranchNavigator) (treeRoot *TreeNode, err error) {
	node := &TreeNode{
		Kind:  libref.ToKind(root),
		Model: root,
	}
	treeRoot = node
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
		list, err := navigator.Next(model)
		if err != nil {
			return err
		}
		for _, m := range list {
			err = walk(m, true)
			if err != nil {
				return err
			}
		}

		return nil
	}
	err = walk(root, false)
	if err != nil {
		return
	}

	return
}

//
// Build the (ancestry) tree.
func (r *Tree) Ancestry(leaf Model, navigator ParentNavigator) (treeRoot *TreeNode, err error) {
	node := &TreeNode{
		Kind:  libref.ToKind(leaf),
		Model: leaf,
	}
	treeRoot = node
	for {
		parent, nErr := navigator.Next(node.Model)
		if nErr != nil {
			err = nErr
			return
		}
		if parent == nil {
			break
		}
		treeRoot = &TreeNode{
			Kind:  libref.ToKind(parent),
			Model: parent,
		}
		treeRoot.Children = append(treeRoot.Children, node)
		node.Parent = treeRoot
		node = treeRoot
	}

	return
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
type ParentNavigator interface {
	Next(Model) (Model, error)
}

//
// Tree navigator.
// Navigate down the children.
type BranchNavigator interface {
	Next(Model) ([]Model, error)
}
