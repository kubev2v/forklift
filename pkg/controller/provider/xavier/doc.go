package xavier

import (
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/virt-controller/pkg/controller/provider/xavier/base"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("xavier")
	base.Log = &log
	Log = &log
}
