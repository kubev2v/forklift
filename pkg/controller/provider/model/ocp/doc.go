package ocp

import "github.com/konveyor/controller/pkg/logging"

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("ocp")
	Log = &log
}

//
// Build all models.
func All() []interface{} {
	return []interface{}{
		&Provider{},
		&NetworkAttachmentDefinition{},
		&StorageClass{},
		&Namespace{},
		&VM{},
	}
}
