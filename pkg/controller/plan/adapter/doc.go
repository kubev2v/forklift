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
	ec2adapter "github.com/kubev2v/forklift/pkg/provider/ec2/controller/adapter"
)

type Adapter = base.Adapter
type Builder = base.Builder
type Ensurer = base.Ensurer
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
	case api.EC2:
		adapter = ec2adapter.New()
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
