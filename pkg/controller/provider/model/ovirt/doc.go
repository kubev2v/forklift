package ovirt

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
)

// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&DataCenter{},
		&Cluster{},
		&ServerCpu{},
		&NICProfile{},
		&DiskProfile{},
		&Network{},
		&StorageDomain{},
		&Disk{},
		&Host{},
		&VM{},
	}
}
