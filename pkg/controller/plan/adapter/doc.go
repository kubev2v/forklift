package adapter

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/vsphere"
)

//
// Adapter factory.
func New(provider *api.Provider) (adapter base.Adapter, err error) {
	//
	switch provider.Type() {
	case api.VSphere:
		adapter = &vsphere.Adapter{}
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
