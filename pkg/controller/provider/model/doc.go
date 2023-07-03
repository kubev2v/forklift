package model

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
)

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
	case api.OVirt:
		all = append(
			all,
			ovirt.All()...)
	case api.OpenStack:
		all = append(
			all,
			openstack.All()...)
	case api.Ova:
		all = append(
			all,
			ova.All()...)
	}

	return
}
