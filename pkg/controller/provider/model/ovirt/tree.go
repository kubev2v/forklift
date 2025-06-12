package ovirt

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Kinds
var (
	DataCenterKind  = libref.ToKind(DataCenter{})
	VNICProfileKind = libref.ToKind(NICProfile{})
	ClusterKind     = libref.ToKind(Cluster{})
	HostKind        = libref.ToKind(Host{})
	NetKind         = libref.ToKind(Network{})
	StorageKind     = libref.ToKind(StorageDomain{})
	DiskKind        = "Disk" // TODO: UPDATE WITH MODEL.
	VmKind          = libref.ToKind(VM{})
	ServerCPUKind   = libref.ToKind(ServerCpu{})
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
