package model

import (
	"github.com/konveyor/controller/pkg/logging"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("model")
	Log = &log
}

//
// Build all models.
func All() []interface{} {
	return []interface{}{
		&Folder{},
		&Datacenter{},
		&Cluster{},
		&Network{},
		&Datastore{},
		&Host{},
		&VM{},
	}
}
