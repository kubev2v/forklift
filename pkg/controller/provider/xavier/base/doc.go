package base

import "github.com/konveyor/controller/pkg/logging"

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("xavier")
	Log = &log
}
