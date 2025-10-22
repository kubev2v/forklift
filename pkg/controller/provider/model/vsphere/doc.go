package vsphere

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
)

// Build all models.
func All() []interface{} {
	return []interface{}{
		&ocp.Provider{},
		&About{},
		&Folder{},
		&Datacenter{},
		&Cluster{},
		&Network{},
		&Datastore{},
		&Host{},
		&VM{},
	}
}
