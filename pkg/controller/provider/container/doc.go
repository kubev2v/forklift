package container

import (
	libcontainer "github.com/konveyor/controller/pkg/inventory/container"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/vsphere"
	core "k8s.io/api/core/v1"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("container")
	Log = &log
}

//
// Build
func Build(
	db libmodel.DB,
	provider *api.Provider,
	secret *core.Secret) libcontainer.Reconciler {
	//
	switch provider.Type() {
	case api.OpenShift:
		ocp.Log = Log
		return ocp.New(db, provider, secret)
	case api.VSphere:
		vsphere.Log = Log
		return vsphere.New(db, provider, secret)
	}

	return nil
}
