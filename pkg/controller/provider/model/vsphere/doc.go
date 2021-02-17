package vsphere

import (
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("vsphere")
	Log = &log
}

//
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
