package ovirt

import (
	"fmt"
	libref "github.com/konveyor/controller/pkg/ref"
)

//
// Kinds
var (
	DataCenterKind  = libref.ToKind(DataCenter{})
	VNICProfileKind = libref.ToKind(VNICProfile{})
	ClusterKind     = libref.ToKind(Cluster{})
	HostKind        = libref.ToKind(Host{})
	NetKind         = libref.ToKind(Network{})
	StorageKind     = libref.ToKind(StorageDomain{})
	VmKind          = libref.ToKind(VM{})
)

//
// Invalid reference.
type InvalidRefError struct {
	Ref
}

func (r InvalidRefError) Error() string {
	return fmt.Sprintf("Reference %#v not valid.", r.Ref)
}
