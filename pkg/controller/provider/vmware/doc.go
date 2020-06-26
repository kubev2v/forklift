package vmware

import (
	"github.com/konveyor/controller/pkg/logging"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("vmware")
	Log = &log
}
