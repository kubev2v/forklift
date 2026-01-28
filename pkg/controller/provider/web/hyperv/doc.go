package hyperv

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovfbase"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.HyperV)
)

// Config for HyperV provider handlers.
var Config = ovfbase.Config{
	ProviderType: api.HyperV,
	Root:         Root,
}

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return ovfbase.Handlers(container, Config)
}
