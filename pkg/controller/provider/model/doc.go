package model

import (
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("vsphere")
	Log = &log
}

//
// All models.
func Models(provider *api.Provider) (all []interface{}) {
	switch provider.Type() {
	case api.VSphere:
		vsphere.Log = Log
		all = append(
			all,
			vsphere.All()...)
	}

	return
}
