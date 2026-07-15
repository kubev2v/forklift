package web

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

const (
	ProviderParam = base.ProviderParam
	Root          = base.ProvidersRoot + "/" + string(api.Azure)
	ProviderRoot  = Root + "/:" + ProviderParam
)

func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{Container: container},
		},
		&VMHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&DiskHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&NetworkHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&StorageHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
	}
}
