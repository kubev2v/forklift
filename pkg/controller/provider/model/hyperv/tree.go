package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Kinds
var (
	ClusterKind = libref.ToKind(Cluster{})
	HostKind    = libref.ToKind(Host{})
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
