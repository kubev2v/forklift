package model

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
)

//
// All models.
func Models(provider *api.Provider) (all []interface{}) {
	switch provider.Type() {
	case api.OpenShift:
		all = append(
			all,
			ocp.All()...)
	case api.VSphere:
		all = append(
			all,
			vsphere.All()...)
	}

	return
}
