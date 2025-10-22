package ocp

import (
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libocp "github.com/kubev2v/forklift/pkg/lib/inventory/container/ocp"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) libcontainer.Collector {
	return &Collector{
		Collector: libocp.New(
			provider,
			secret,
			&Namespace{
				log: logging.WithName("collection|namespace").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			},
			&NetworkAttachmentDefinition{
				log: logging.WithName("collection|network").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			},
			&StorageClass{
				log: logging.WithName("collection|storageclass").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			},
			&InstanceType{
				log: logging.WithName("collection|instancetype").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			},
			&ClusterInstanceType{
				log: logging.WithName("collection|clusterinstancetype").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			},
			&VM{
				log: logging.WithName("collection|vm").WithValues(
					"provider",
					path.Join(
						provider.GetNamespace(),
						provider.GetName())),
			}),
	}
}

// NO-OP
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// OCP collector.
type Collector struct {
	*libocp.Collector
}
