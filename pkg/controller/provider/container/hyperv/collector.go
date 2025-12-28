package hyperv

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/ovfbase"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
)

// New creates a HyperV collector using the shared OVF-based collector logic.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *ovfbase.Collector {
	return ovfbase.New(db, provider, secret, "hyperv")
}
