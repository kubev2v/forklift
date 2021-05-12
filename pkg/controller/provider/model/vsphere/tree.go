package vsphere

import (
	libref "github.com/konveyor/controller/pkg/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
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
// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
