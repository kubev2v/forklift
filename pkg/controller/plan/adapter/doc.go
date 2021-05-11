package adapter

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/vsphere"
)

type Adapter = base.Adapter
type Builder = base.Builder
type Validator = base.Validator

//
// Adapter factory.
func New(provider *api.Provider) (adapter Adapter, err error) {
	//
	switch provider.Type() {
	case api.VSphere:
		adapter = &vsphere.Adapter{}
	case api.OVirt:
		adapter = &ovirt.Adapter{}
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
