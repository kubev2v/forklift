package model

import (
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("model")
	Log = &log
}

//
// All models.
func Models(provider *api.Provider) (all []interface{}) {
	switch provider.Type() {
	case api.OpenShift:
		ocp.Log = Log
		all = append(
			all,
			ocp.All()...)
	case api.VSphere:
		vsphere.Log = Log
		all = append(
			all,
			vsphere.All()...)
	}

	return
}
