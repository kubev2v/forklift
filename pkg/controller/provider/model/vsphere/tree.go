package vsphere

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

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

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
