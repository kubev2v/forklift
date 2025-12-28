package ovf

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Kinds
var (
	VmKind      = libref.ToKind(VM{})
	NetKind     = libref.ToKind(Network{})
	DiskKind    = libref.ToKind(Disk{})
	StorageKind = libref.ToKind(Storage{})
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
