package vsphere

import (
	"github.com/konveyor/controller/pkg/logging"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("vsphere")
	Log = &log
}
