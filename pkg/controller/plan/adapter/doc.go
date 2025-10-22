package adapter

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/ocp"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/openstack"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/ova"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
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
