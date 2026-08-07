package nutanix

import (
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
)

// Nutanix (AHV) has no ESXi-style Host resources or host-based direct disk
// transfer; no host-level operations needed.
type Handler struct {
	*handler.Handler
}

func (r *Handler) Watch(_ *handler.WatchManager) (err error) {
	return
}
