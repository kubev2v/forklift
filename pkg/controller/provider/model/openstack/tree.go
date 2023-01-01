package openstack

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
)

// Kinds
var (
	VMKind       = libref.ToKind(VM{})
	ProjectKind  = libref.ToKind(Project{})
	ImageKind    = libref.ToKind(Image{})
	FlavorKind   = libref.ToKind(Flavor{})
	VolumeKind   = libref.ToKind(Volume{})
	SnapshotKind = libref.ToKind(Snapshot{})
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode
type BranchNavigator = base.BranchNavigator
type ParentNavigator = base.ParentNavigator
