package openstack

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Kinds
var (
	RegionKind     = libref.ToKind(Region{})
	ProjectKind    = libref.ToKind(Project{})
	ImageKind      = libref.ToKind(Image{})
	FlavorKind     = libref.ToKind(Flavor{})
	VMKind         = libref.ToKind(VM{})
	SnapshotKind   = libref.ToKind(Snapshot{})
	VolumeKind     = libref.ToKind(Volume{})
	VolumeTypeKind = libref.ToKind(VolumeType{})
	NetworkKind    = libref.ToKind(Network{})
	SubnetKind     = libref.ToKind(Subnet{})
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
