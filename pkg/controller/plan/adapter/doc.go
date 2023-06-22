package adapter

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/vsphere"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

type Adapter = base.Adapter
type Builder = base.Builder
type Client = base.Client
type Validator = base.Validator
type DestinationClient = base.DestinationClient

// Adapter factory.
func New(provider *api.Provider) (adapter Adapter, err error) {
	//
	switch provider.Type() {
	case api.VSphere:
		adapter = &vsphere.Adapter{}
	case api.OVirt:
		adapter = &ovirt.Adapter{}
	case api.OpenStack:
		adapter = &openstack.Adapter{}
	case api.OpenShift:
		adapter = &ocp.Adapter{}
	case api.Ova:
		adapter = &ova.Adapter{}
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
