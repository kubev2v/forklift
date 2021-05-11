package ovirt

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
)

//
// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&DataCenter{},
		&Cluster{},
		&VNICProfile{},
		&Network{},
		&StorageDomain{},
		&Host{},
		&VM{},
	}
}
